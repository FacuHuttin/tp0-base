# TP0: Docker + Comunicaciones + Concurrencia

### Ejercicio N°1:
Definir un script de bash `generar-compose.sh` que permita crear una definición de Docker Compose con una cantidad configurable de clientes. El nombre de los containers deberá seguir el formato propuesto: client1, client2, client3, etc. 

El script deberá ubicarse en la raíz del proyecto y recibirá por parámetro el nombre del archivo de salida y la cantidad de clientes esperados:

`./generar-compose.sh docker-compose-dev.yaml 5`

Considerar que en el contenido del script pueden invocar un subscript de Go o Python:

```
#!/bin/bash
echo "Nombre del archivo de salida: $1"
echo "Cantidad de clientes: $2"
python3 mi-generador.py $1 $2
```

En el archivo de Docker Compose de salida se pueden definir volúmenes, variables de entorno y redes con libertad, pero recordar actualizar este script cuando se modifiquen tales definiciones en los sucesivos ejercicios.

### Implementación

#### Archivos creados:

* generar-compose.sh

* generador-compose.py

#### Aspectos importantes

##### generar-compose.sh

Este archivo se utiliza de la siguiente manera:

`./generar-compose.sh docker-compose-dev.yaml <cant. de clientes>`

Y solamente le pasa por parametro al generador-compose.py la cantidad de clientes y el nombre de archivo de salida.

##### generador-compose.py

Este archivo de python está encargado de crear el archivo docker-compose.
Se utilizó las siguientes funciones para obtener la cadena de texo de cada parte del archivo:

* generate_base_content()
 - Devuelve la cadena de texto del encabezado del archivo junto con el servicio del servidor

* generate_network_content()
 - Devuelve la cadena de texto de la red

* generate_client_content(client_id)
 - Devuelve la cadena de texto especifica para el servicio del cliente <<client_id>>

Luego de obtener las partes, se concatenan y se escribe en la direccion del archivo pasado por parametro. 