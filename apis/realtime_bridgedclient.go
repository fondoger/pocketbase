package apis

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/subscriptions"
)

type IBridgedClient interface {
	subscriptions.Client

	ClientSubscription() *ClientSubscription

	// IsRemoteClient returns true if the client is connected to current pocketbase instance via WebSocket.
	// It returns false if the client is connected to other pocketbase instances.
	IsRemoteClient() bool

	// BroadcastGoOffline notifies all other pocketbase instances that the client is offline.
	BroadcastGoOffline()

	// BroadcastChanges notifies all other pocketbase instances about the client's subscription changes.
	// It also works as BroadcastGoOnline() when it is called for the first time.
	BroadcastChanges()

	// ReceiveChanges receives subscription changes from other pocketbase instances.
	ReceiveChanges(newSubscription *ClientSubscription, authRecordJson string)
}

// ClientSubscription is stored in the _realtime_clients table in PostgreSQL.
// Every time a new client connects to the pocketbase server, a new row is created.
// And other pocketbase servers are notified via Postgres's Listen/Notify feature.
type ClientSubscription struct {
	ClientId           string   `db:"pk,client_id" json:"client_id"`
	ChannelId          string   `db:"channel_id" json:"channel_id"`
	Subscriptions      []string `db:"subscriptions" json:"subscriptions"`
	ClientAuthId       string   `db:"client_auth_id" json:"client_auth_id"`
	UpdatedByChannelId string   `db:"updated_by_channel_id" json:"updated_by_channel_id"`
}

var _ subscriptions.Client = (*BridgedClient)(nil)
var _ IBridgedClient = (*BridgedClient)(nil)

// BridgedClient defines a remote subscription client.
type BridgedClient struct {
	subscriptions.Client
	bridge       IRealtimeBridge
	subscription *ClientSubscription
}

// NewBridgedClient
// - NewBridgedClient(bridge) creates a local client
// - NewBridgedClient(bridge, optionalSubscription) creates a remote client
func NewBridgedClient(bridge IRealtimeBridge, optionalSubscription ...*ClientSubscription) *BridgedClient {
	var client subscriptions.Client
	var subscription *ClientSubscription

	if len(optionalSubscription) > 0 && optionalSubscription[0] != nil {
		// remote client (client is connected to other pocketbase instance)
		client = subscriptions.NewDefaultClient(optionalSubscription[0].ClientId)
		subscription = optionalSubscription[0]
	} else {
		// local client (client is connected to current pocketbase instance)
		client = subscriptions.NewDefaultClient()
		subscription = &ClientSubscription{
			ClientId:           client.Id(),
			ChannelId:          bridge.SelfChannelId(),
			Subscriptions:      []string{},
			ClientAuthId:       "",
			UpdatedByChannelId: "",
		}
	}

	return &BridgedClient{
		bridge:       bridge,
		Client:       client,
		subscription: subscription,
	}
}

func (r *BridgedClient) BroadcastGoOffline() {
	// delete and notify
	_, err := r.bridge.App().DB().NewQuery(`
		WITH deleted AS (
			DELETE FROM _realtime_clients
			WHERE client_id = {:client_id}
			RETURNING *
		)
		SELECT pg_notify('shared_bridge_channel', 'subscription_delete|' || deleted.client_id::text) FROM deleted;
	`).Bind(dbx.Params{
		"client_id": r.subscription.ClientId,
	}).Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error deleting subscription:", err)
		return
	}
}

// BroadcastChanges notifies all other pocketbase instances about the client's subscription changes.
// If it is called for the first time, it will act as `BroadcastGoOnline()`.
// If the client has an auth record, it will send the auth record JSON to other pocketbase instances.
func (r *BridgedClient) BroadcastChanges() {
	if len(r.RawSubscriptions()) == 0 {
		// treat no subscriptions as offline
		r.BroadcastGoOffline()
		return
	}

	// 1. clientId should not be changed
	// 2. channelId should not be changed
	// 3. update subscriptions
	r.subscription.Subscriptions = r.RawSubscriptions()
	// 4. updatedByChannelId is set to the current channelId
	r.subscription.UpdatedByChannelId = r.bridge.SelfChannelId()

	// 5. update auth id
	var authSQL string
	var authParams dbx.Params
	if record, _ := r.Get(RealtimeClientAuthKey).(*core.Record); record != nil {
		r.subscription.ClientAuthId = record.TableName() + "/" + record.Id
		authSQL = fmt.Sprintf(`SELECT * FROM {{%s}} WHERE id = {:auth_record_id} LIMIT 1`, record.TableName())
		authParams = dbx.Params{
			"auth_record_id": record.Id,
		}
	} else {
		r.subscription.ClientAuthId = ""
		authSQL = "SELECT WHERE 1=0" // empty rows.
	}

	// Update using raw SQL query
	_, err := r.bridge.App().DB().NewQuery(fmt.Sprintf(`
		WITH
			updated AS (
				INSERT INTO _realtime_clients (
					client_id, channel_id, subscriptions, client_auth_id, updated_by_channel_id
				) VALUES (
					{:client_id}, {:channel_id}, {:subscriptions}, {:client_auth_id}, {:updated_by_channel_id}
				) ON CONFLICT (client_id) DO UPDATE
				SET subscriptions = EXCLUDED.subscriptions,
					client_auth_id = EXCLUDED.client_auth_id,
					updated_by_channel_id = EXCLUDED.updated_by_channel_id
				RETURNING *
			),
			auth AS (
				%s
			)
		SELECT pg_notify(
			'shared_bridge_channel', 
			CONCAT_WS(
				'|',
				'subscription_upsert', 
				(SELECT row_to_json(u) FROM updated u)::text, 
				COALESCE((SELECT row_to_json(a) FROM auth a)::text, '')
			)
		);
	`, authSQL)).Bind(authParams).Bind(dbx.Params{
		"client_id":             r.subscription.ClientId,
		"channel_id":            r.subscription.ChannelId,
		"subscriptions":         r.subscription.Subscriptions,
		"client_auth_id":        r.subscription.ClientAuthId,
		"updated_by_channel_id": r.subscription.UpdatedByChannelId,
	}).Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error updating subscription:", err)
		return
	}
	// other sibling servers will be notified by Postgres Listen/Notify feature
}

func (r *BridgedClient) ReceiveChanges(newSubscription *ClientSubscription, authRecordJson string) {
	// 1. clientId should not be changed
	// 2. channelId should not be changed
	// 3. update subscriptions
	r.Client.Unsubscribe() // clear previous subscriptions
	r.Client.Subscribe(newSubscription.Subscriptions...)
	// 4. update auth record
	if newSubscription.ClientAuthId == "" {
		r.Client.Unset(RealtimeClientAuthKey)
	} else {
		authRecord, err := AuthRecordFromJson(r.bridge.App(), newSubscription.ClientAuthId, authRecordJson)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error unmarshalling auth record JSON:", err)
			return
		}
		r.Client.Set(RealtimeClientAuthKey, authRecord)
	}
	// 5. updatedByChannelId should not be changed

	// Finally, update the subscription
	r.subscription = newSubscription
}

// Send sends the specified message to the client's channel (if not discarded).
func (r *BridgedClient) Send(m subscriptions.Message) {
	if r.IsDiscarded() {
		return
	}

	if r.IsRemoteClient() {
		// send to sibling servers
		r.bridge.SendViaBridge(r.subscription.ChannelId, r.Id(), m)
	} else {
		// send to connected clients
		r.Client.Send(m)
	}
}

func (r *BridgedClient) IsRemoteClient() bool {
	return r.subscription.ChannelId != r.bridge.SelfChannelId()
}

func (r *BridgedClient) ClientSubscription() *ClientSubscription {
	return r.subscription
}

func AuthRecordFromJson(app core.App, clientAuthId string, authRecordJson string) (*core.Record, error) {
	if authRecordJson == "" {
		return nil, nil
	}
	collectionName, _, ok := split2(clientAuthId, "/") // format: collectionName/recordId
	if !ok {
		return nil, fmt.Errorf("invalid client auth ID: %s", clientAuthId)
	}
	collection, err := app.FindCachedCollectionByNameOrId(collectionName)
	if err != nil {
		return nil, fmt.Errorf("error finding collection by name or ID: %w", err)
	}
	authRecord := core.NewRecord(collection)

	data := make(map[string]any)
	_ = json.Unmarshal([]byte(authRecordJson), &data)
	for _, field := range collection.Fields {
		fieldName := field.GetName()
		value, _ := field.PrepareValue(authRecord, data[fieldName])
		authRecord.SetRaw(fieldName, value)
	}
	return authRecord, nil
}

func split2(s string, sep string) (string, string, bool) {
	parts := strings.SplitN(s, sep, 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func split3(s string, sep string) (string, string, string, bool) {
	parts := strings.SplitN(s, sep, 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
