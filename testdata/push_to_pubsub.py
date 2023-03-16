import os, sys, json, uuid
from datetime import datetime, timezone, timedelta
from google.cloud import pubsub_v1
from typing import Callable
from concurrent import futures

project_id = os.environ["PUBSUB_PROJECT"]
topic_id = os.environ["PUBSUB_TOPIC"]

publisher = pubsub_v1.PublisherClient()
topic_path = publisher.topic_path(project_id, topic_id)

def get_callback(
    publish_future: pubsub_v1.publisher.futures.Future, data: str
) -> Callable[[pubsub_v1.publisher.futures.Future], None]:
    def callback(publish_future: pubsub_v1.publisher.futures.Future) -> None:
        try:
            # Wait 60 seconds for the publish call to succeed.
            print(publish_future.result(timeout=60))
        except futures.TimeoutError:
            print(f"Publishing {data} timed out.")
    return callback

publish_futures = []

for i in range(1000):
    data = {
        "timestamp": int(datetime.now(timezone.utc).timestamp() * 1e6),
        "id": str(uuid.uuid4()),
        "prop1": "prop1",
        "prop2": "prop2"
    }
    publish_future = publisher.publish(topic_path, json.dumps(data).encode("utf-8"))
    #publish_future.add_done_callback(get_callback(publish_future, data))
    publish_futures.append(publish_future)

# Wait for all the publish futures to resolve before exiting.
futures.wait(publish_futures, return_when=futures.ALL_COMPLETED)

print(f"Published {len(publish_futures)} messages to {topic_path}.")
