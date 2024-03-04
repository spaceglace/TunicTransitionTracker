The backend for the Tunic Transition Tracker (tm)

Still very much a work in progress, but will get updated here as things solidify

### API
`GET /` static directory to /frontend in the same directory, for storing a frontend webpage

`GET /spoiler` returns main json blob generated from latest save file and spoiler.log

`GET /nospoiler` attempts to recreate most of the information without using the spoiler.log. Currently unfinished.

`GET /settings` returns json of current settings file

`POST /settings` takes in a json blob to write as the new settings file