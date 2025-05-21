/// <reference path="types.d.ts" />

console.log("Loading realtime hooks...");

// A new user is online, notify all existing clients about the online user count
onRealtimeSubscribeRequest((e) => {
  console.log("Realtime subscribe request", e);
  e.next();

  if (!e.client.hasSubscription("online-users")) {
    return;
  }
  // notify all clients about the online user count
  const users = {};
  const clients = $app.subscriptionsBroker().clients();
  for (let clientId in clients) {
    if (!clients[clientId].hasSubscription("online-users")) {
      continue;
    }
    const auth = clients[clientId].get("auth");
    if (!auth) {
      continue;
    }
    users[auth.id] = true;
  }
  const message = new SubscriptionMessage({
    name: "online-users",
    data: JSON.stringify({ onlineUserCount: Object.keys(users).length }),
  });
  for (let clientId in clients) {
    if (!clients[clientId].hasSubscription("online-users")) {
      continue;
    }
    clients[clientId].send(message);
  }
});

// A user is offline, notify all existing clients about the online user count
onRealtimeConnectRequest((e) => {
  e.next();

  // Ignore clients that do not have the "online-users" subscription
  if (!e.client.hasSubscription("online-users")) {
    return;
  }

  const users = {};
  const clients = $app.subscriptionsBroker().clients();
  for (let clientId in clients) {
    if (!clients[clientId].hasSubscription("online-users")) {
      continue;
    }
    const auth = clients[clientId].get("auth");
    if (!auth) {
      continue;
    }
    users[auth.id] = true;
  }
  const message = new SubscriptionMessage({
    name: "online-users",
    data: JSON.stringify({ onlineUserCount: Object.keys(users).length }),
  });
  for (let clientId in clients) {
    if (!clients[clientId].hasSubscription("online-users")) {
      continue;
    }
    clients[clientId].send(message);
  }
});