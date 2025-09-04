# TP0: Docker + Comunicaciones + Concurrencia

### Ejercicio N°8:

Modificar el servidor para que permita aceptar conexiones y procesar mensajes en paralelo. En caso de que el alumno implemente el servidor en Python utilizando _multithreading_,  deberán tenerse en cuenta las [limitaciones propias del lenguaje](https://wiki.python.org/moin/GlobalInterpreterLock).

### Implementación:

#### Archivos modificados:

* main.go

* client.go

* central.py

* server.py

#### Aspectos importantes

##### server.py

* Se agregó 2 locks de hilos como atributos de la clase `Server`:
  * `_shutdown_lock`, lockea la sección crítica de solicitar el shutdown del servidor al recibir la señal SIGTERM.
  * `_threads_lock`, lockea la sección crítica de manejo de los hilos de los clientes (agregado en la lista de `active_threads` y limpieza de los hilos si finalizaron) 

* Se optó por utilizar 1 hilo por cliente dado que en este TP se tiene una cantidad chica de clientes y que las operaciones que se realiza para cada hilo es de I/O heavy, es decir, que la mayoria del tiempo durante las conexiones o se almacena las apuestas o se espera al resto de los clientes para que se inicie la loteria. 

##### central.py

* Se agregó 2 locks de hilos como atributos de la clase `Central`:
  * `_state_lock`, lockea la sección crítica de obtener el resultado de la loteria y de la ejecución de la loteria. 
  * `_storage_lock`, lockea la sección crítica de almacenamiento de apuestas. 