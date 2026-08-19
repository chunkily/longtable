#!/bin/bash

cd web && npm run build && cd ..
go build -tags nodynamic -o longtable ./cmd/longtable
