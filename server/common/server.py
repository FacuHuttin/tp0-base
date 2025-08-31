import socket
import logging
import signal
from .transport import TCPConnection
from .central import process_communication

TIMEOUT = 1.0

# Global shutdown flag
shutdown_requested = False

def handle_shutdown(signum, frame):
    global shutdown_requested
    logging.info(f'action: shutdown_signal_received | result: in_progress')
    shutdown_requested = True

class Server:
    def __init__(self, port, listen_backlog, timeout=TIMEOUT):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._server_socket.settimeout(timeout)

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """

        while not shutdown_requested:
            try:
                client_sock = self.__accept_new_connection()
                if client_sock:
                    self.__handle_client_connection(client_sock)
            except socket.timeout:
                continue
            except OSError as e:
                if shutdown_requested:
                    logging.info('action: server_shutdown | result: in_progress')
                else:
                    logging.error(f'action: accept_error | error: {e}')
        self._server_socket.close()
        logging.info('action: server_exit | result: success')

    def __handle_client_connection(self, client_sock: socket.socket):
        """
        Creates a connection object and processes communication
        following the protocol for bet messages and ACK responses
        """
        connection = None
        
        try:
            connection = TCPConnection(client_sock)
            
            addr = connection.get_address()
            logging.info(f'action: client_connection_established | result: success | ip: {addr[0]}')
            
            success = process_communication(connection)
            
            if success:
                logging.info(f'action: client_communication | result: success | ip: {addr[0]}')
            else:
                logging.error(f'action: client_communication | result: fail | ip: {addr[0]}')
                
        except Exception as e:
            addr = client_sock.getpeername() if client_sock else ('unknown', 0)
            logging.error(f"action: handle_client_connection | result: fail | ip: {addr[0]} | error: {e}")
        finally:
            # Clean up connection
            if connection:
                connection.close()
            elif client_sock:
                try:
                    client_sock.close()
                except:
                    pass

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """
        
        try:
            logging.info('action: accept_connections | result: in_progress')
            c, addr = self._server_socket.accept()
            logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
            return c
        except socket.timeout:
            return None
        except OSError:
            if shutdown_requested:
                return None
            raise

# Register signal handler
signal.signal(signal.SIGTERM, handle_shutdown)