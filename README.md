# TP0: Docker + Comunicaciones + Concurrencia

### Ejercicio N°3:
Crear un script de bash `validar-echo-server.sh` que permita verificar el correcto funcionamiento del servidor utilizando el comando `netcat` para interactuar con el mismo. Dado que el servidor es un echo server, se debe enviar un mensaje al servidor y esperar recibir el mismo mensaje enviado.

En caso de que la validación sea exitosa imprimir: `action: test_echo_server | result: success`, de lo contrario imprimir:`action: test_echo_server | result: fail`.

El script deberá ubicarse en la raíz del proyecto. Netcat no debe ser instalado en la máquina _host_ y no se pueden exponer puertos del servidor para realizar la comunicación (hint: `docker network`). `

### Implementación:

#### Archivo creado:

* validar-echo-server.sh

#### Aspectos importantes

* Este script se utiliza de la siguiente manera:

`./validar-echo-server.sh`

Y simplesmente ejecuta las siguientes linea de comando:

```
RESPONSE=$(docker run --rm --network $NETWORK_NAME alpine sh -c "echo $TEST_MESSAGE | nc $SERVER_CONTAINER_NAME $SERVER_PORT")

if [ "$RESPONSE" = "$TEST_MESSAGE" ]; then
  RESULT="success"
fi

echo "action: test_echo_server | result: $RESULT"
```

* Se optó por utilizar la imagen de alpine dado que es super liviana ( 7.8MB ) y ya viene instalado con netcat.
