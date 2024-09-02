import sys

def generate_base_content():
    return f"""name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
      - LOGGING_LEVEL=DEBUG
    volumes:
      - ./server/config.ini:/config.ini
    networks:
      - testing_net
      
"""

def generate_client_content(client_id):
    return f"""  client{client_id}:
    container_name: client{client_id}
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID={client_id}
      - CLI_LOG_LEVEL=DEBUG
    volumes:
      - ./client/config.yaml:/config.yaml
    networks:
      - testing_net
    depends_on:
      - server

"""

def generate_network_content():
    return """networks:
  testing_net:
    ipam:
      driver: default
      config:
        - subnet: 172.25.125.0/24
"""

def generate_docker_compose(output_file, num_clients):

  base_content = generate_base_content()
  network_content = generate_network_content()

  clients_content = ""
  if num_clients > 0:
    clients_content = "".join(generate_client_content(i) for i in range(1, num_clients + 1))

  full_content = base_content + clients_content + network_content

  with open(output_file, 'w') as f:
    f.write(full_content)

if __name__ == "__main__":
  if len(sys.argv) != 3:
    print("Usage: python3 generador_compose.py <output_file> <number_of_clients>")
    sys.exit(1)

  output_file = sys.argv[1]
  num_clients = int(sys.argv[2])

  generate_docker_compose(output_file, num_clients)