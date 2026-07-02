#!/bin/bash

BASE_URL="http://localhost:8080"
ORIGINAL_URL="https://practicum.yandex.ru/"

echo "------------------------------------------------------"
echo "POST $ORIGINAL_URL [shorten a url]"
echo "+++"

SHORT=$(curl -s -X POST "$BASE_URL/" \
  -H "Content-Type: text/plain" \
  -d $ORIGINAL_URL)

echo "Short URL: $SHORT"

echo "------------------------------------------------------"
echo "GET $SHORT [resolve the shortened url]"
echo "+++"
curl -v "$SHORT"
