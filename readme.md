# pubsub-to-bigquery

An example about how stream messages from Pub/Sub to BigQuery 
using the new [BigQuery Storage Write API](https://cloud.google.com/bigquery/docs/write-api)

## Why

This can be used a seed project and can be deployed on CloudRun when you need to stream data form Pub/Sub to BigQuery and 
[BigQuery subscriptions](https://cloud.google.com/pubsub/docs/bigquery) solution is not 
enough because you need to do some transformations and [DataFlow](https://cloud.google.com/dataflow)
is way too complex to deploy and manage.

## How to build and run

Setup your variables 

```bash
PROJECT_ID=test-244421
```

Create PubSub topic and subscription

```bash
TOPIC_ID=demo-events
gcloud pubsub topics create $TOPIC_ID --project $PROJECT_ID
gcloud pubsub subscriptions create $TOPIC_ID-sub --topic=$TOPIC_ID --project $PROJECT_ID
```

Create BigQuery table from schema

```bash
# Create the dataset
DATASET_ID=demo_bq_dataset
bq --location=us-central1 mk \
    --dataset \
     $PROJECT_ID:$DATASET_ID

bq mk \
    --table \
    $PROJECT_ID:$DATASET_ID.demo_events \
    model/bq_schema.json
```

Run local

```bash
export LOG_LEVEL=debug
export PORT=8080
export TRACE_SAMPLE="1"
export PUBSUB_PROJECT=$PROJECT_ID
export PUBSUB_TOPIC="demo-events"
export PUBSUB_SUBSCRIPTION="demo-events-sub"
export BIGQUERY_PROJECT=$PROJECT_ID
export BIGQUERY_DATASET="demo_bq_dataset"
export BIGQUERY_TABLE="demo_events"

go run main.go
```

Test

```bash
cd testdata
python3 push_to_pubsub.py
```

Deploy on GCP

## TODO:

- [ ] Benchmark streaming performance
- [ ] Add counters and metrics and save them to monitoring
- [ ] Add tests for Pub/Sub handler and BigQuery sink

## License

This project is licensed under the terms of the MIT license.
