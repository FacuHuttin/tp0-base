import socket
import logging
import signal
from .utils import Bet
from .scheduler import Scheduler
from .constants import ERROR_MESSAGE_ID, ACK_MESSAGE_ID, WINNERS_MESSAGE_ID, BET_BATCH_MESSAGE_ID

class ServerShutdownError(Exception):
    pass

class Server:
    def __init__(self, port, listen_backlog, max_amount_of_agencies, national_lottery_center):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._shutdown_flag = False
        self.national_lottery_center = national_lottery_center
        self.max_amount_of_agencies = max_amount_of_agencies
        self.scheduler = Scheduler()

        signal.signal(signal.SIGTERM, self._handle_sigterm)

    def _handle_sigterm(self, signum, frame):
        self._shutdown_flag = True

    def run(self):

        while not self._shutdown_flag:
            if self.scheduler.amount_of_agencies() == self.max_amount_of_agencies:
                if self.scheduler.all_bets_stored():
                    logging.info("action: sorteo | result: success")
                    winners_dict = self.national_lottery_center.get_winners()
                    while self.scheduler.amount_of_agencies() > 0:
                        agency = self.scheduler.pop_agency()
                        agency.winners = winners_dict.get(agency.id, [])
                        self.__handle_client_connection(agency, sending_winners=True)
                        agency.socket.close()
                    break
                else:
                    next_agency = self.scheduler.get_next_agency()
                    if next_agency:
                        self.__handle_client_connection(next_agency)
            else:
                try:
                    self._server_socket.settimeout(1.0)
                    client_sock = self.__accept_new_connection()
                    if client_sock:
                        self.scheduler.add_agency(client_sock)
                        next_agency = self.scheduler.get_next_agency()
                        if next_agency:
                            self.__handle_client_connection(next_agency)
                except socket.timeout:
                    if self.scheduler.amount_of_agencies() == 0:
                        continue
                    next_agency = self.scheduler.get_next_agency()
                    if next_agency:
                        self.__handle_client_connection(next_agency)
        self._server_socket.close()
        if self._shutdown_flag:
            while self.scheduler.amount_of_agencies() > 0:
                agency = self.scheduler.pop_agency()
                agency.socket.close()
        logging.info("action: close_server_socket | result: success")

    def __handle_client_connection(self, agency, sending_winners=False):
        if sending_winners:
            try:
                if self._shutdown_flag:
                    raise ServerShutdownError("Server is shutting down")
                addr = agency.socket.getpeername()

                data = bytearray()

                # Append the WINNERS_MESSAGE_ID (1 byte)
                data.extend(WINNERS_MESSAGE_ID.to_bytes(1, byteorder='big'))

                # Append the amount of winners (1 byte)
                data.extend(len(agency.winners).to_bytes(1, byteorder='big'))

                # Append each winner as uint32 (4 bytes each)
                winners = agency.winners
                for winner in winners:
                    data.extend(int(winner).to_bytes(4, byteorder='big'))

                if self._shutdown_flag:
                    raise ServerShutdownError("Server is shutting down")
                self.__send_all_to_socket(agency.socket, data)

                self.__read_confirmation_message(agency)

                logging.info(f'action: ganadores_enviados | result: success | cantidad: {len(winners)}')
            except ServerShutdownError:
                logging.info(f"action: client_socket_shutdown | result: success | ip: {addr[0]}")
            except OSError as e:
                if self._shutdown_flag:
                    logging.info(f"action: client_socket_shutdown | result: success | ip: {addr[0]}")
                else:
                    logging.error(f"action: receive_message | result: fail | error: {e}")
        else:
            try:
                if self._shutdown_flag:
                    raise ServerShutdownError("Server is shutting down")
                addr = agency.socket.getpeername()
                if self._shutdown_flag:
                    raise ServerShutdownError("Server is shutting down")
                bets, bets_amount, is_last_batch, agency_id = self.__read_batch_message(agency.socket)
                if len(bets) == 0:
                    error_code = bytes([ERROR_MESSAGE_ID])
                    self.__send_all_to_socket(agency.socket, error_code)
                    agency.socket.close()
                    return
                logging.info(f'action: apuesta_recibida | result: success | cantidad: {bets_amount}')
                if not agency.id:
                    agency.id = agency_id
                byte_msg = bytes([ACK_MESSAGE_ID])

                if self._shutdown_flag:
                    raise ServerShutdownError("Server is shutting down")
                self.__send_all_to_socket(agency.socket, byte_msg)

                if self._shutdown_flag:
                    raise ServerShutdownError("Server is shutting down")
                self.national_lottery_center.store_bets_from_agency(bets)

                logging.info(f'action: apuestas_almacenadas | result: success | cantidad: {bets_amount}')
                if is_last_batch:
                    agency.bets_stored = True
            except ServerShutdownError:
                logging.info(f"action: client_socket_shutdown | result: success | ip: {addr[0]}")
            except OSError as e:
                logging.error(f"action: receive_message | result: fail | error: {e}")

    def __read_confirmation_message(self, agency):
        try:
            byte_message_id = self.__recv_all_from_socket(agency.socket, 1)
            if not byte_message_id:
                raise ValueError("Failed to read message_id from confirm message")
            message_id = int.from_bytes(byte_message_id, byteorder='big')
            if message_id != ACK_MESSAGE_ID:
                raise ValueError(f"Invalid message id, expected {ACK_MESSAGE_ID} but received {message_id}")
        except Exception as e:
            logging.error(f'action: read_confirmation_message | result: fail | error: {e}')

    def __read_batch_message(self, client_sock):
        try:
            byte_message_id = self.__recv_all_from_socket(client_sock, 1)
            if not byte_message_id:
                raise ValueError("Failed to read message_id from message")
            message_id = int.from_bytes(byte_message_id, byteorder='big')

            if message_id != BET_BATCH_MESSAGE_ID:
                raise ValueError(f"Invalid message id, expected {BET_BATCH_MESSAGE_ID} but received {message_id}")

            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")

            byte_agency_id = self.__recv_all_from_socket(client_sock, 1)
            if not byte_agency_id:
                raise ValueError("Failed to read agency_id from message")
            agency_id = int.from_bytes(byte_agency_id, byteorder='big')

            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")
            
            byte_last_bets_batch = self.__recv_all_from_socket(client_sock, 1)
            if not byte_last_bets_batch:
                raise ValueError("Failed to read last_bets_batch flag from message")
            last_bets_batch: bool = int.from_bytes(byte_last_bets_batch, byteorder='big')

            if self._shutdown_flag:
                raise ServerShutdownError("Server is shutting down")

            byte_bets_amount = self.__recv_all_from_socket(client_sock, 1)
            if not byte_bets_amount:
                raise ValueError("Failed to read bets amount from message")
            bets_amount = int.from_bytes(byte_bets_amount, byteorder='big')

            bets = []

            for _ in range(0, bets_amount):
                if self._shutdown_flag:
                    raise ServerShutdownError("Server is shutting down")
                bets.append(self.__read_bet_message(client_sock, agency_id))
            
            return bets, bets_amount, last_bets_batch, agency_id
        except ServerShutdownError:
            raise
        except Exception as e:
            logging.error(f'action: apuesta_recibida | result: fail | cantidad: {bets_amount} | error: {e}')
            return [], bets_amount, last_bets_batch, agency_id

    def __read_bet_message(self, client_sock, agency_id):

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
