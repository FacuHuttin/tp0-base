# TP0: Docker + Comunicaciones + Concurrencia

### Ejercicio N°2:
Modificar el cliente y el servidor para lograr que realizar cambios en el archivo de configuración no requiera reconstruír las imágenes de Docker para que los mismos sean efectivos. La configuración a través del archivo correspondiente (`config.ini` y `config.yaml`, dependiendo de la aplicación) debe ser inyectada en el container y persistida por fuera de la imagen (hint: `docker volumes`).

### Implementación:

#### Archivo modificado:

* generador-compose.py

#### Aspectos importantes

* Se eliminó las variables de entorno de loggueo en las secciones de *server* y *client*

* Se agregó la subsección *volumes* dentro de los servicios de *server* y *client*
  * Para el *server*:
   ./server/config.ini:/config.ini
  * Para el *client*:
    ./client/config.yaml:/config.yaml

Con estas modificaciones se puede realizar cambios en los archivos de configuración sin la necesidad de reconstruír las imágenes de Docker para que los mismos sean efectivos.