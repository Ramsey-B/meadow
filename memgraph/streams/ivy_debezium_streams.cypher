CALL mg.load_all();

CREATE KAFKA STREAM ivy_merged_entities
TOPICS "ivy.public.merged_entities"
TRANSFORM ivy_debezium.transform_merged_entity
BOOTSTRAP_SERVERS "kafka:29092"
BATCH_INTERVAL 100
BATCH_SIZE 10
CONFIGS { "auto.offset.reset": "earliest" };

CREATE KAFKA STREAM ivy_merged_relationships
TOPICS "ivy.public.merged_relationships"
TRANSFORM ivy_debezium.transform_merged_relationship
BOOTSTRAP_SERVERS "kafka:29092"
BATCH_INTERVAL 100
BATCH_SIZE 10
CONFIGS { "auto.offset.reset": "earliest" };

START STREAM ivy_merged_entities;
START STREAM ivy_merged_relationships;


