# TP0: Docker + Comunicaciones + Concurrencia

### Ejercicio N°5:
Modificar la lógica de negocio tanto de los clientes como del servidor para nuestro nuevo caso de uso.

#### Cliente
Emulará a una _agencia de quiniela_ que participa del proyecto. Existen 5 agencias. Deberán recibir como variables de entorno los campos que representan la apuesta de una persona: nombre, apellido, DNI, nacimiento, numero apostado (en adelante 'número'). Ej.: `NOMBRE=Santiago Lionel`, `APELLIDO=Lorca`, `DOCUMENTO=30904465`, `NACIMIENTO=1999-03-17` y `NUMERO=7574` respectivamente.

Los campos deben enviarse al servidor para dejar registro de la apuesta. Al recibir la confirmación del servidor se debe imprimir por log: `action: apuesta_enviada | result: success | dni: ${DNI} | numero: ${NUMERO}`.

#### Servidor
Emulará a la _central de Lotería Nacional_. Deberá recibir los campos de la cada apuesta desde los clientes y almacenar la información mediante la función `store_bet(...)` para control futuro de ganadores. La función `store_bet(...)` es provista por la cátedra y no podrá ser modificada por el alumno.
Al persistir se debe imprimir por log: `action: apuesta_almacenada | result: success | dni: ${DNI} | numero: ${NUMERO}`.

#### Comunicación:
Se deberá implementar un módulo de comunicación entre el cliente y el servidor donde se maneje el envío y la recepción de los paquetes, el cual se espera que contemple:
* Definición de un protocolo para el envío de los mensajes.
* Serialización de los datos.
* Correcta separación de responsabilidades entre modelo de dominio y capa de comunicación.
* Correcto empleo de sockets, incluyendo manejo de errores y evitando los fenómenos conocidos como [_short read y short write_](https://cs61.seas.harvard.edu/site/2018/FileDescriptors/).

### Implementación:

#### Archivos modificados:

* main.go

* client.go

* server.py

#### Archivos creados:

* Servidor:
  
  * central.py
  * protocol.py
  * transport.py

* Cliente:

  * agency.go
  * protocol.go
  * transport.go

#### Aspectos importantes

##### Protocolo:

Se utilizó un protocolo de 3 mensajes (Msg de Apuesta, Msg de Recibo, Msg de Cierre) que funciona de la siguiente manera:
  1. El cliente se conecta al servidor y envia el Msg de Apuesta.
  2. El servidor recibe el mensaje, lo procesa y envia el Msg de Recibo.
  3. El cliente recibe el mensaje y envia el Msg de Cierre y cierra la conexión.
  4. El servidor recibe este ultimo mensaje y cierra la conexión

* Msg de Apuesta:
  * 1er byte: el tipo del mensaje (1: Msg de Apuesta, 2: Msg de Recibo, 3: Msg de Cierre)
  * 2do byte: el numero de la agencia (número del cliente)
  * Desde el 3ro byte hasta el final se utiliza:
    * el esquema de largo (1 byte) + contenido para los campos variables (Nombre y Apellido del apostador), 
    * para los campos fijos (Cumpleaños y DNI) simplesmente se envia encodeados los caracteres en su codigo ASCII
    * para el campo de número de apuesta se convierte en un numero de 32 bits y se envia en formato big endian

* Msg de Recibo:
  * 1er byte: el tipo del mensaje (1: Msg de Apuesta, 2: Msg de Recibo, 3: Msg de Cierre)
  * 2do byte: el numero de la agencia (número del cliente)
  * 3ro-10do byte: el DNI encodeado
  * 11do-14do byte: el número de apuesta en numero de 32-bit big endian

* Msg de Cierre:
  * 1er byte: el tipo del mensaje (1: Msg de Apuesta, 2: Msg de Recibo, 3: Msg de Cierre)

##### main.go

* Se agregó las variables de entorno:
  * name
  * surname
  * dni
  * birthday
  * betnumber

  Con la finalidad de almacenarlo en la nueva estructura AgencyInfo y enviarlo por medio de mensajes al servidor.

##### client.go

* Se agregó los campos en la estructura `ClientConfig`:
  * Name
  * Surname
  * DNI
  * Birthday
  * BetNumber

* Se agregó el atributo `service` que es de tipo `AgencyService` en la estructura `Client`

* Se modificó el método `StartClientLoop()` de la estructura `Client` con lo siguiente:
  * Delegado el proceso de comunicacion con el servidor al `AgencyService`

##### agency.go

* Agregado de la estructura `AgencyService` que tiene la funcionalidad de realizar y mantener la comunicación con el servidor.

* Este tiene los atributos:
  * `AgencyInfo`, estructura que contiene los datos de la Agencia de apuestas (Numero de la Agencia y los datos de la apuesta)
  * `Connection`, interface de la estructura `TCPConnection` que maneja el socket de comunicación con el servidor.

* Tiene los siguientes métodos:
  * `ProcessCommunication(msgID)`, realiza la comunicacion con el servidor
  * `CreateBetMessage(messageID)`, crea el Msg de Apuesta
  * `SendMessage(message)`, envia el mensaje al servidor
  * `ReceiveAckMessage()`, recibe el Msg de Recibo
  * `SendCloseMessage()`, envia el Msg de Cierre

##### protocol.go

* En este archivo se ubican las funciones relacionadas al protocolo. (Serialización y Deserialización de campos de los mensajes enviados y recibidos) Las más importantes son:
  * `SerializeBetMessage()`, devuelve el mensaje en un arreglo de bytes.
  * `DeserializeAckBetMessage()`, devuelve la estructura `AckBetMessage` (esta tiene como campos el numero de la agencia, el dni del apostador y el numero de la apuesta)
  * `SendBetMessage(connection)`, envia el Msg de Apuesta al servidor
  * `CreateCloseMessage()`, crea el Msg de Cierre
  * `SendCloseMessage(connection)`, envia el Msg de Cierre al servidor

##### transport.go

* En este archivo se ubica la estructura de conexión y su interfaz (`TCPConnection` y `Connection`)

* La estructura tiene los siguientes atributos:
  * `conn net.Conn`, conexión con el servidor
  * `address string`, dirección del servidor

* También tiene los siguientes metodos:
  * `Connect()`, se conecta con el servidor
  * `Send(message []byte)`, envia los bytes del mensaje utilizando un loop con un contador. Este solamente termina hasta que todos los bytes fueron enviados.
  * `ReceiveExactBytes(count int)`, recibe `count` cantidad de bytes. Este también tiene un loop y un contador que solo termina si se obtuvo la cantidad total de bytes pedidos.
  * `Close()`, cierra la conexión con el servidor


##### server.py

* Se delegó el proceso de comunicación con los clientes a la función `process_communication()` ubicada en el archivo central.py

##### central.py

* En este archivo se ubica las funciones:
  * `process_communication()`, recibe el Msg de Apuesta, procesa el mensaje (llama la función `process_bet_message()`), envía el Msg de Recibo, recibe el Msg de Cierre y termina la conexión.
  * `process_bet_message()`, almacena la apuesta en el archivo bets.csv, crea y serializa el Msg de Recibo, y devuelve el Msg de Recibo.

* Las funciones mencionadas utilizan las funciones auxiliares de protocolo ubicadas en el archivo `protocolo.py`

##### protocolo.py

* Este archivo es la otra parte del protocolo de comunicación. En este archivo se encuentra las funciones:
  * `serialize_ack_bet_message(data)`
  * `deserialize_bet_message(data)`
  * `create_ack_message(Bet)`
  * `receive_bet_message_from_connection(connection)`
  * `send_ack_message_to_connection(connection)`

##### transport.py

* Este archivo es igual que `transport.go`, tiene la interface `Connection` y la clase `TCPConnection` con los siguientes métodos:
  * `receive_exact_bytes(count)`
  * `send(data: bytes)`
  * `close(count)`
  * `get_address(count)`