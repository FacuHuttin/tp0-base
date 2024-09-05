import socket
import logging
import signal
from .utils import Bet

class ServerShutdownError(Exception):
    pass

class Server:
    def __init__(self, port, listen_backlog, national_lottery_center):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._shutdown_flag = False
        self.national_lottery_center = national_lottery_center

        signal.signal(signal.SIGTERM, self._handle_sigterm)

    def _handle_sigterm(self, signum, frame):
        self._shutdown_flag = True

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """

        while not self._shutdown_flag:
            try:
                self._server_socket.settimeout(1.0)
                client_sock = self.__accept_new_connection()
                if client_sock:
                    self.__handle_client_connection(client_sock)
            except socket.timeout:
                continue

        self._server_socket.close()
        logging.info("action: close_server_socket | result: success")

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        addr = client_sock.getpeername()

        try:
            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")
            bets, bets_amount = self.__read_batch_message(client_sock)
            if len(bets) == 0:
                error_code = bytes([2])  # Example error code for reading failure
                self.__send_all_to_socket(client_sock, error_code)
                client_sock.close()
                return
            logging.info(f'action: apuesta_recibida | result: success | cantidad: {bets_amount}')
            
            byte_msg = bytes([1])
            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")
            self.__send_all_to_socket(client_sock, byte_msg)
            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")
            
            self.national_lottery_center.store_bets_from_agency(bets)
            logging.info(f'action: apuestas_almacenadas | result: success | cantidad: {bets_amount}')
        except ServerShutdownError:
            logging.info(f"action: client_socket_shutdown | result: success | ip: {addr[0]}")
        except OSError as e:
            logging.error(f"action: receive_message | result: fail | error: {e}")
        finally:
            client_sock.close()

    def __read_batch_message(self, client_sock):
        try:
            bytes_bets_amount = self.__recv_all_from_socket(client_sock, 1)
            if not bytes_bets_amount:
                raise ValueError("Failed to read bets amount from message")
            bets_amount = int.from_bytes(bytes_bets_amount, byteorder='big')

            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")

            bytes_agency_id = self.__recv_all_from_socket(client_sock, 1)
            if not bytes_agency_id:
                raise ValueError("Failed to read agency_id from message")
            agency_id = int.from_bytes(bytes_agency_id, byteorder='big')

            bets = []

            for _ in range(0, bets_amount):
                if self._shutdown_flag:
                    raise ServerShutdownError("Server is shutting down")
                bets.append(self.__read_bet_message(client_sock, agency_id))
            
            return bets, bets_amount
        except ServerShutdownError:
            raise
        except Exception as e:
            logging.error(f'action: apuesta_recibida | result: fail | cantidad: {bets_amount} | error: {e}')
            return [], bets_amount

    def __read_bet_message(self, client_sock, agency_id):

        if self._shutdown_flag:
            raise ServerShutdownError("Server is shutting down")
        bytes_name_length = self.__recv_all_from_socket(client_sock, 1)
        if not bytes_name_length:
            raise ValueError("Failed to read name length from message")
        name_length = int.from_bytes(bytes_name_length, byteorder='big')

        if self._shutdown_flag:
            raise ServerShutdownError("Server is shutting down")
        bytes_name = self.__recv_all_from_socket(client_sock, name_length)
        if not bytes_name:
            raise ValueError("Failed to read name from message")
        name = bytes_name.decode('utf-8')

        if self._shutdown_flag:
            raise ServerShutdownError("Server is shutting down")
        bytes_surname_length = self.__recv_all_from_socket(client_sock, 1)
        if not bytes_surname_length:
            raise ValueError("Failed to read surname length from message")
        surname_length = int.from_bytes(bytes_surname_length, byteorder='big')

        if self._shutdown_flag:
            raise ServerShutdownError("Server is shutting down")
        bytes_surname = self.__recv_all_from_socket(client_sock, surname_length)
        if not bytes_surname:
            raise ValueError("Failed to read surname from message")
        surname = bytes_surname.decode('utf-8')

        if self._shutdown_flag:
            raise ServerShutdownError("Server is shutting down")
        bytes_document = self.__recv_all_from_socket(client_sock, 4)
        if not bytes_document:
            raise ValueError("Failed to read document from message")
        document = int.from_bytes(bytes_document, byteorder='big')

        if self._shutdown_flag:
            raise ServerShutdownError("Server is shutting down")
        bytes_birthday = self.__recv_all_from_socket(client_sock, 10)
        if not bytes_birthday:
            raise ValueError("Failed to read birthday from message")
        birthday = bytes_birthday.decode('utf-8')

        if self._shutdown_flag:
            raise ServerShutdownError("Server is shutting down")
        bytes_lottery_num = self.__recv_all_from_socket(client_sock, 2)
        if not bytes_lottery_num:
            raise ValueError("Failed to read document from message")
        lottery_num = int.from_bytes(bytes_lottery_num, byteorder='big')

        return Bet(agency_id, name, surname, document, birthday, lottery_num)


    def __send_all_to_socket(self, sock, data):
        total_sent = 0
        while total_sent < len(data):
            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")
            sent = sock.send(data[total_sent:])
            if sent == 0:
                raise RuntimeError("Socket connection broken")
            total_sent += sent

    def __recv_all_from_socket(self, sock, length):
        data = bytearray()
        while len(data) < length:
            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")
            packet = sock.recv(length - len(data))
            if not packet:
                return None
            data.extend(packet)
        return data

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """

        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        try:
            c, addr = self._server_socket.accept()
            logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
            return c
        except socket.timeout:
            return None
        except OSError as e:
            if self._shutdown_flag:
                return None 
            logging.error(f"action: accept_connections | result: fail | error: {e}")
            return None
