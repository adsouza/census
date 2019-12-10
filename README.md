# Census

A simple CRUD webapp for academic libraries to gather stats on usage of physical space.

# Dependencies

This app is written in Go 1.13 & runs on Google App Engine so you will need to [download & install the Google Cloud SDK for Go](https://cloud.google.com/appengine/docs/standard/go/download) to work on it.
Having done so, you can run the app locally using the usual command: `go run main.go` after setting the GOOGLE_APPLICATION_CREDENTIALS env var to the location of your GCP service account key file.

# Deployment

```
gcloud app deploy
```
