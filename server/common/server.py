import socket
import logging
import signal
import threading

from .transport import TCPConnection
from .central import Central

class AgencyHandler(threading.Thread):
    """Thread to handle communication with a single agency connection"""
    
    def __init__(self, client_socket: socket.socket, central: Central, server_ref):
        super().__init__()
        self.client_socket = client_socket
        self.central = central
        self.server_ref = server_ref
        self.connection = None
        self.daemon = True
        
    def run(self):
        try:
            addr = self.client_socket.getpeername()
            logging.debug(f'action: agency_handler | result: in_progress | ip: {addr[0]}')
            
            self.connection = TCPConnection(self.client_socket, self.server_ref.timeout, self.server_ref)
            success = self.central.process_communication(self.connection, self.server_ref)
            
            if success:
                logging.debug(f'action: agency_communication | result: success | ip: {addr[0]}')
            else:
                if not self.server_ref.shutdown_requested:
                    logging.error(f'action: agency_communication | result: fail | ip: {addr[0]}')
                    
        except Exception as e:
            if not self.server_ref.shutdown_requested:
                addr = self.client_socket.getpeername() if self.client_socket else ('unknown', 0)
                logging.error(f'action: agency_handler | result: fail | ip: {addr[0]} | error: {e}')
        finally:
            self.cleanup()
    
    def cleanup(self):
        try:
            if self.connection:
                self.connection.close()
            elif self.client_socket:
                self.client_socket.close()
        except Exception as e:
            logging.error(f'action: agency_handler_cleanup | result: fail | error: {e}')

class Server:
    def __init__(self, port, listen_backlog, timeout, total_agencies):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._server_socket.settimeout(timeout)
        
        self.timeout = timeout
        self.total_agencies = total_agencies
        
        # Instance-based shutdown flag and lock for thread safety
        self.shutdown_requested = False
        self._shutdown_lock = threading.Lock()
        
        # Create central instance for managing lottery logic
        self.central = Central(total_agencies)
        
        # List to keep track of active agency handler threads
        self.active_threads = []
        self._threads_lock = threading.Lock()
        
        # Register signal handlers
        signal.signal(signal.SIGTERM, self._handle_shutdown)
        signal.signal(signal.SIGINT, self._handle_shutdown)
    
    def _handle_shutdown(self, signum, frame):
        with self._shutdown_lock:
            if not self.shutdown_requested:
                logging.info(f'action: shutdown_signal_received | result: in_progress | signal: {signum}')
                self.shutdown_requested = True
                
                try:
                    self._server_socket.close()
                except:
                    pass

    def run(self):
        """
        Multithreaded Server that creates one thread per client connection
        Can handle up to total_agencies concurrent connections
        """
        try:            
            connections_handled = 0
            
            while not self.shutdown_requested and connections_handled < self.total_agencies:
                try:
                    # Accept new connection
                    client_sock = self._accept_new_connection()
                    
                    if self.shutdown_requested:
                        if client_sock:
                            client_sock.close()
                        break
                        
                    if client_sock:
                        # Create a new thread to handle this agency
                        handler = AgencyHandler(client_sock, self.central, self)
                        
                        # Add to active threads list
                        with self._threads_lock:
                            self.active_threads.append(handler)
                        
                        handler.start()
                        connections_handled += 1
                        
                        logging.info(f'action: agency_connection | result: success | connections_handled: {connections_handled}/{self.total_agencies}')
                        
                except socket.timeout:
                    self._cleanup_finished_threads()
                    continue
                except OSError as e:
                    if self.shutdown_requested:
                        logging.debug('action: server_socket_closed_during_shutdown | result: in_progress')
                        break
                    else:
                        logging.error(f'action: accept_error | result: fail | error: {e}')
                        break
            
            self._wait_for_all_threads_to_finish()
            
        except Exception as e:
            logging.error(f'action: server_error | result: fail | error: {e}')
        finally:
            self._cleanup()
    
    def _accept_new_connection(self):
        try:
            logging.debug('action: accept_connections | result: in_progress')
            client_sock, addr = self._server_socket.accept()
            logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
            return client_sock
        except socket.timeout:
            return None
        except OSError:
            if self.shutdown_requested:
                return None
            raise
    
    def _cleanup_finished_threads(self):
        with self._threads_lock:
            self.active_threads = [t for t in self.active_threads if t.is_alive()]
    
    def _wait_for_all_threads_to_finish(self):
        logging.info('action: waiting_for_all_threads | result: in_progress')
        
        with self._threads_lock:
            threads_to_wait = self.active_threads.copy()
        
        for thread in threads_to_wait:
            if thread.is_alive():
                thread.join()
        
        logging.info('action: all_threads_finished | result: success')
    
    def _cleanup(self):
        logging.info('action: server_cleanup | result: in_progress')
        
        with self._shutdown_lock:
            self.shutdown_requested = True
        
        try:
            self._server_socket.close()
        except:
            pass
        
        with self._threads_lock:
            active_threads = self.active_threads.copy()
        
        for thread in active_threads:
            if thread.is_alive():
                thread.join(timeout=2.0)
        
        logging.info('action: server_shutdown | result: success')