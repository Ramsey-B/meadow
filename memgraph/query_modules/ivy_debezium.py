import json
import mgp
from typing import Any, Dict, List, Optional


# Memgraph Kafka Streams transformation module.
#
# This module uses the mgp (Memgraph Python) API to register transformation
# procedures that process Debezium CDC events from Postgres.
#
# Topics expected (from Debezium connector `topic.prefix=ivy`):
# - ivy.public.merged_entities
# - ivy.public.merged_relationships


def _as_obj(message: Any) -> Optional[Dict[str, Any]]:
    if message is None:
        return None
    if isinstance(message, (bytes, bytearray)):
        try:
            message = message.decode("utf-8")
        except Exception:
            return None
    if isinstance(message, str):
        try:
            return json.loads(message)
        except Exception:
            return None
    if isinstance(message, dict):
        return message
    return None


def _payload(obj: Dict[str, Any]) -> Dict[str, Any]:
    # Debezium output may be wrapped as {"payload": {...}} depending on converter settings.
    if "payload" in obj and isinstance(obj["payload"], dict):
        return obj["payload"]
    return obj


def _cypher_escape_string(s: str) -> str:
    return s.replace("\\", "\\\\").replace("'", "\\'")


def _cypher_literal(v: Any) -> str:
    if v is None:
        return "NULL"
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float)):
        return str(v)
    if isinstance(v, str):
        return "'" + _cypher_escape_string(v) + "'"
    if isinstance(v, (list, tuple)):
        return "[" + ", ".join(_cypher_literal(x) for x in v) + "]"
    # For objects/maps, store JSON string
    try:
        return "'" + _cypher_escape_string(json.dumps(v, separators=(",", ":"))) + "'"
    except Exception:
        return "'" + _cypher_escape_string(str(v)) + "'"


def _set_props(var: str, props: Dict[str, Any], skip: Optional[set] = None) -> str:
    skip = skip or set()
    clauses: List[str] = []
    for k, v in props.items():
        if k in skip:
            continue
        # Cypher property keys must be valid identifiers; fall back to JSON string if not.
        if not k.replace("_", "").isalnum():
            continue
        clauses.append(f"SET {var}.{k} = {_cypher_literal(v)}")
    return "\n".join(clauses)


def _build_entity_cypher(message_value: bytes) -> Optional[str]:
    obj = _as_obj(message_value)
    if obj is None:
        return None

    p = _payload(obj)
    op = p.get("op")
    after = p.get("after")
    before = p.get("before")

    row = after if isinstance(after, dict) else before if isinstance(before, dict) else None
    if row is None:
        return None

    entity_id = row.get("id")
    tenant_id = row.get("tenant_id")
    entity_type = row.get("entity_type")
    data = row.get("data") or {}

    if not entity_id or not tenant_id or not entity_type:
        return None

    # Parse data if it's a JSON string
    if isinstance(data, str):
        try:
            data = json.loads(data)
        except Exception:
            data = {}

    label = str(entity_type)
    props: Dict[str, Any] = {}
    props["id"] = str(entity_id)
    props["tenant_id"] = str(tenant_id)
    props["entity_type"] = str(entity_type)
    for k in ("version", "source_count", "created_at", "updated_at", "deleted_at"):
        if k in row:
            props[k] = row.get(k)

    # Merge in data fields (flatten).
    if isinstance(data, dict):
        for k, v in data.items():
            props[k] = v

    cypher = f"""
MERGE (e:{label} {{id: {_cypher_literal(props["id"])}, tenant_id: {_cypher_literal(props["tenant_id"])}}})
{_set_props("e", props)}
RETURN e;
""".strip()
    return cypher


def _build_relationship_cypher(message_value: bytes) -> Optional[str]:
    obj = _as_obj(message_value)
    if obj is None:
        return None

    p = _payload(obj)
    after = p.get("after")
    before = p.get("before")
    row = after if isinstance(after, dict) else before if isinstance(before, dict) else None
    if row is None:
        return None

    tenant_id = row.get("tenant_id")
    rel_type = row.get("relationship_type")
    rel_id = row.get("id")
    from_id = row.get("from_merged_entity_id")
    to_id = row.get("to_merged_entity_id")
    from_type = row.get("from_entity_type")
    to_type = row.get("to_entity_type")
    data = row.get("data") or {}

    if not tenant_id or not rel_type or not rel_id or not from_id or not to_id or not from_type or not to_type:
        return None

    # Parse data if it's a JSON string
    if isinstance(data, str):
        try:
            data = json.loads(data)
        except Exception:
            data = {}

    props: Dict[str, Any] = {}
    props["id"] = str(rel_id)
    props["tenant_id"] = str(tenant_id)
    for k in ("source_plan_id", "source_execution_id", "created_at", "updated_at", "deleted_at"):
        if k in row:
            props[k] = row.get(k)
    if isinstance(data, dict):
        for k, v in data.items():
            props[k] = v

    cypher = f"""
MERGE (a:{from_type} {{id: {_cypher_literal(str(from_id))}, tenant_id: {_cypher_literal(str(tenant_id))}}})
MERGE (b:{to_type} {{id: {_cypher_literal(str(to_id))}, tenant_id: {_cypher_literal(str(tenant_id))}}})
MERGE (a)-[r:{rel_type} {{id: {_cypher_literal(props["id"])}, tenant_id: {_cypher_literal(props["tenant_id"])}}}]->(b)
{_set_props("r", props)}
RETURN r;
""".strip()
    return cypher


@mgp.transformation
def transform_merged_entity(messages: mgp.Messages) -> mgp.Record(query=str, parameters=mgp.Nullable[mgp.Map]):
    """Transform Debezium CDC events for merged_entities into Cypher queries."""
    result = []
    for i in range(messages.total_messages()):
        msg = messages.message_at(i)
        payload = msg.payload()
        cypher = _build_entity_cypher(payload)
        if cypher:
            result.append(mgp.Record(query=cypher, parameters=None))
    return result


@mgp.transformation
def transform_merged_relationship(messages: mgp.Messages) -> mgp.Record(query=str, parameters=mgp.Nullable[mgp.Map]):
    """Transform Debezium CDC events for merged_relationships into Cypher queries."""
    result = []
    for i in range(messages.total_messages()):
        msg = messages.message_at(i)
        payload = msg.payload()
        cypher = _build_relationship_cypher(payload)
        if cypher:
            result.append(mgp.Record(query=cypher, parameters=None))
    return result
