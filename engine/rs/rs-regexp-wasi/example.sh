#!/bin/bash

jq -c -n '{
	pattern: "^helo",
	text: "helo,wrld",
}' |
	wazero run ./rs-regexp-wasi.wasm |
	jq
