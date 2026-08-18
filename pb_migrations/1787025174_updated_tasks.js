/// <reference path="../pb_data/types.d.ts" />
migrate((app) => {
  const collection = app.findCollectionByNameOrId("pbc_2602490748")

  // update field
  collection.fields.addAt(4, new Field({
    "help": "",
    "hidden": false,
    "id": "select1655102503",
    "maxSelect": 1,
    "name": "priority",
    "presentable": false,
    "required": false,
    "system": false,
    "type": "select",
    "values": [
      "high",
      "low",
      "medium"
    ]
  }))

  // update field
  collection.fields.addAt(5, new Field({
    "help": "",
    "hidden": false,
    "id": "file2036324795",
    "maxSelect": 10,
    "maxSize": 0,
    "mimeTypes": null,
    "name": "attachment",
    "presentable": false,
    "protected": false,
    "required": false,
    "system": false,
    "thumbs": null,
    "type": "file"
  }))

  return app.save(collection)
}, (app) => {
  const collection = app.findCollectionByNameOrId("pbc_2602490748")

  // update field
  collection.fields.addAt(4, new Field({
    "help": "",
    "hidden": false,
    "id": "select1655102503",
    "maxSelect": 0,
    "name": "priority",
    "presentable": false,
    "required": false,
    "system": false,
    "type": "select",
    "values": [
      "high"
    ]
  }))

  // update field
  collection.fields.addAt(5, new Field({
    "help": "",
    "hidden": false,
    "id": "file2036324795",
    "maxSelect": 0,
    "maxSize": 0,
    "mimeTypes": null,
    "name": "attachment",
    "presentable": false,
    "protected": false,
    "required": false,
    "system": false,
    "thumbs": null,
    "type": "file"
  }))

  return app.save(collection)
})
