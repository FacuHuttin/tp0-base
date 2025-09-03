import socket
import logging
import signal
from .transport import TCPConnection
from .central import Central

class Server:
    def __init__(self, port, listen_backlog, timeout, total_agencies):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._server_socket.settimeout(timeout)

        self.timeout = timeout
        # Instance-based shutdown flag
        self.shutdown_requested = False
        
        # Create central instance for managing lottery logic
        self.central = Central(total_agencies)
        
        # Register signal handlers
        signal.signal(signal.SIGTERM, self._handle_shutdown)
    
    def _handle_shutdown(self, signum, frame):
        """Handle shutdown signal"""
        logging.info(f'action: shutdown_signal_received | result: in_progress')
        self.shutdown_requested = True

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """

        while not self.shutdown_requested:
            try:
                client_sock = self.__accept_new_connection()
                if self.shutdown_requested:
                    logging.info('action: server_shutdown | result: in_progress')
                    break
                if client_sock:
                    self.__handle_client_connection(client_sock)
            except socket.timeout:
                continue
            except OSError as e:
                if self.shutdown_requested:
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
            # Check if shutdown was requested before starting
            if self.shutdown_requested:
                logging.info('action: client_connection_skipped | result: shutdown_requested')
                return
                
            connection = TCPConnection(client_sock, self.timeout, self)
            
            addr = connection.get_address()
            logging.info(f'action: client_connection_established | result: success | ip: {addr[0]}')
            
            success = self.central.process_communication(connection, self)
            
            if success:
                logging.info(f'action: client_communication | result: success | ip: {addr[0]}')
            else:
                if self.shutdown_requested:
                    logging.info(f'action: client_communication | result: shutdown | ip: {addr[0]}')
                else:
                    logging.error(f'action: client_communication | result: fail | ip: {addr[0]}')
                
        except Exception as e:
            addr = client_sock.getpeername() if client_sock else ('unknown', 0)
            if self.shutdown_requested:
                logging.info(f"action: handle_client_connection | result: shutdown | ip: {addr[0]} | info: {e}")
            else:
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
            if self.shutdown_requested:
                return None
            raise