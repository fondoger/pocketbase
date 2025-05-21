# Realtime Chat demo with Pocketbase JS SDK.

File to edit: `index.html`

1. Each time a new user open the website, a new user is created in the database.
2. All users use PocketBase realtime API to listen for new messages.

**UI**

- A list of messages.
- A form to send a new message.
- A button to send the message.
- Display fake user names and avatars.
- I want simple UIs like apple messages app or shadcn styles.
- Use aminial emoji as avatars!
- Users can use dice to change their name and avatar.

**Realtime SDK Example**

```js
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

...

// (Optionally) authenticate
await pb.collection('users').authWithPassword('test@example.com', '1234567890');

// Subscribe to changes in any record in the collection
pb.collection('example').subscribe('*', function (e) {
    console.log(e.action);
    console.log(e.record);
}, { /* other options like expand, custom headers, etc. */ });


// Unsubscribe
pb.collection('example').unsubscribe('*'); // remove all '*' topic subscriptions
```
