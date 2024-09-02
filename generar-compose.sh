#!/bin/bash

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <output_file> <number_of_clients>"
  exit 1
fi

OUTPUT_FILE=$1
NUM_CLIENTS=$2

echo "Nombre del archivo de salida: $OUTPUT_FILE"
echo "Cantidad de clientes: $NUM_CLIENTS"

python3 generador_compose.py "$OUTPUT_FILE" "$NUM_CLIENTS"