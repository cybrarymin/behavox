#!/bin/bash

API_URL="https://127.0.0.1:443/v1/events"
source .envrc

print_usage() {
    echo "Usage: $0 <event_type>  [additional arguments]"
    echo "  metric: $0 metric <value>"
    echo "  log:    $0 log <level> <message>"
    exit 1
}

if [[ $# -lt 2 ]]; then
    print_usage
fi

EVENT_TYPE="$1"
EVENT_ID=`uuidgen`

case "$EVENT_TYPE" in
    metric)
        if [[ $# -ne 2 ]]; then print_usage; fi
        
        VALUE="$2"
        JSON_PAYLOAD=$(jq -n --arg id "$EVENT_ID" --arg value "$VALUE" \
            '{event: {event_type: "metric", event_id: $id, value: ($value | tonumber)}}')
            
        ;;
    log)
        if [[ $# -ne 3 ]]; then print_usage; fi
        LEVEL="$2"
        MESSAGE="$3"
        JSON_PAYLOAD=$(jq -n --arg id "$EVENT_ID" --arg level "$LEVEL" --arg message "$MESSAGE" \
            '{event: {event_type: "log", event_id: $id, level: $level, message: $message}}')
        ;;
    *)
        print_usage
        ;;
esac


TOKEN=$(curl -k -X POST -u "$ADMINUSER:$ADMINPASS" https://localhost:443/v1/tokens | jq .result.token | tr -d '"')


if [ -z $TOKEN ] || [[ $TOKEN == *"error"* ]]; then
    echo "Authentication failed. Could not obtain token."
    exit 1
fi

curl -k -X POST -H "Authorization: Bearer $TOKEN" "$API_URL" -H "Content-Type: application/json" -d "$JSON_PAYLOAD"
