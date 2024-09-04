import sys
import os

def load_env_file(env_file):
    with open(env_file) as f:
        for line in f:
            if line.strip() and not line.startswith('#'):
                key, value = line.strip().split('=', 1)
                os.environ[key] = value

def generate_base_content():
    return f"""name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
    volumes:
      - ./server/config.ini:/config.ini
    networks:
      - testing_net
      
"""

def generate_client_content(client_id):

    name = os.getenv(f'CLIENT{client_id}_NAME', 'default_name')
    surname = os.getenv(f'CLIENT{client_id}_SURNAME', 'default_surname')
    dni = os.getenv(f'CLIENT{client_id}_DNI', '00000000')
    birthday = os.getenv(f'CLIENT{client_id}_BIRTHDAY', '0000-00-00')
    lottery_num = os.getenv(f'CLIENT{client_id}_LOTTERYNUM', '0000')

    return f"""  client{client_id}:
    container_name: client{client_id}
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID={client_id}
      - CLI_NAME={name}
      - CLI_SURNAME={surname}
      - CLI_DNI={dni}
      - CLI_BIRTHDAY={birthday}
      - CLI_LOTTERYNUM={lottery_num}
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

  load_env_file('.env')

  generate_docker_compose(output_file, num_clients)