# TP0: Docker + Comunicaciones + Concurrencia

### Ejercicio N°4:
Modificar servidor y cliente para que ambos sistemas terminen de forma _graceful_ al recibir la signal SIGTERM. Terminar la aplicación de forma _graceful_ implica que todos los _file descriptors_ (entre los que se encuentran archivos, sockets, threads y procesos) deben cerrarse correctamente antes que el thread de la aplicación principal muera. Loguear mensajes en el cierre de cada recurso (hint: Verificar que hace el flag `-t` utilizado en el comando `docker compose down`).

### Implementación:

#### Archivos modificados:

* main.go

* client.go

* main.py

* server.py

#### Aspectos importantes

##### main.go

* Se creó un channel de señales del sistema operativo: `signalChan: chan os.Signal` para que notifique cuando se obtiene la señal SIGTERM. Este se pasó por parámetro al método `StartClientLoop()` de la estructura `Client`

##### client.go

* Se modificó el método `StartClientLoop()` de la estructura `Client` con lo siguiente:
  * Agregado de verificación al comienzo del loop en caso de que se recibió la señal del channel 

  * Reemplazo de la linea:

  ```
  time.Sleep(c.config.LoopPeriod)
  ```

  con

  ```
  	sleepInterval := 100 * time.Millisecond
		totalSleep := time.Duration(0)
		for totalSleep < c.config.LoopPeriod {
			select {
			case sig := <-signalChan:
				log.Infof("action: signal_received | result: success | client_id: %v | completed_msg_id: %v | signal: %v",
					c.config.ID, msgID, sig)
				return
			case <-time.After(sleepInterval):
				totalSleep += sleepInterval
			}
		}
  ```

  Este cambio hace con que mientras se esté "durmiendo" se pueda detectar a cada 100 milisegundos si hubo una señal del sistema operativo. Si no hubo se sigue con el "Sleep"

##### main.py

* Se agregó la variable de entorno de configuración SERVER_TIMEOUT para que se pase por parámetro a la clase "Server"

##### server.py

* Se agregó un timeout al socket de aceptación de conexiones con la finalidad de detectar las señales del sistema operativo (SIGTERM)

* Se agregó el método _handle_shutdown a la clase "Server" para setear el atributo shutdown_requested en True. (Este método se pasa por parámetro a la funcion signal.signal() para que sea llamada si se detecta la señal de terminación)