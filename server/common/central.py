import logging
import threading

from .utils import store_bets, load_bets, has_won
from .transport import TCPConnection, ShutdownRequestedError
from .protocol import (
    MESSAGE_TYPE_BET, MESSAGE_TYPE_CLOSE,
    MESSAGE_TYPE_NOTIFICATION, MESSAGE_TYPE_WINNERS_REQUEST,
    deserialize_batch_bet_message, create_batch_ack_message, receive_message_type,
    receive_batch_bet_message_from_connection,
    create_notification_response, create_winners_not_available_response,
    create_winners_available_response, receive_winners_request
)


class Central:
    """
    Manages the lottery state and agency completion tracking.
    Handles all communication protocol logic for bet processing and winner queries.
    Thread-safe implementation with proper synchronization.
    """
    
    def __init__(self, total_agencies: int):
        self.agencies_completed = set()
        self.lottery_completed = False
        self.total_agencies = total_agencies
        self.agency_winners_dict = {}
        # Lock for thread-safe access to shared state
        self._state_lock = threading.Lock()
        # Lock for thread-safe access to storage functions
        self._storage_lock = threading.Lock()
            
    def process_agency_completion(self, agency_number: int) -> bool:
        with self._state_lock:
            self.agencies_completed.add(agency_number)
            logging.info(f"action: agency_completion | result: success | agency: {agency_number} | completed_agencies: {len(self.agencies_completed)} | total_agencies: {self.total_agencies}")
            
            if len(self.agencies_completed) == self.total_agencies:
                return self.conduct_lottery()
            return False
    
    def conduct_lottery(self) -> bool:
        """
        Conduct the lottery when all agencies have completed sending bets
        Returns True if lottery was conducted, False if already conducted
        NOTE: This method should be called while holding the _state_lock
        """
        if not self.lottery_completed:
            self.lottery_completed = True
            
            with self._storage_lock:
                for bet in load_bets():
                    if has_won(bet):
                        if bet.agency not in self.agency_winners_dict:
                            self.agency_winners_dict[bet.agency] = []
                        self.agency_winners_dict[bet.agency].append(bet.document)
            
            logging.info("action: sorteo | result: success")
            
            return True
        
        return False
    
    def is_lottery_completed(self) -> bool:
        with self._state_lock:
            return self.lottery_completed
    
    def get_agency_winners(self, agency_number: int) -> list[str]:
        try:
            with self._state_lock:
                winners = self.agency_winners_dict.get(agency_number, [])
            
            logging.info(f"action: winners_query | result: success | agency: {agency_number} | winners_count: {len(winners)}")
            return winners
            
        except Exception as e:
            logging.error(f"action: winners_query | result: fail | agency: {agency_number} | error: {e}")
            return []
    
    def process_batch_bet_message(self, data: bytes):
        try:
            batch_number, bets = deserialize_batch_bet_message(data)
            
            with self._storage_lock:
                store_bets(bets)

            agency_number = bets[0].agency if bets else 0
            ack_message = create_batch_ack_message(agency_number, batch_number)
            ack_data = ack_message.serialize_ack_bet_message()
            
            logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
            
            return ack_data
            
        except Exception as e:
            logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)} | error: {e}")
            raise
    
    def process_communication(self, connection: TCPConnection, server=None):
        """
        Handles the communication protocol for batch betting and winner queries:
        1. Receive batch bet message -> send ACK -> repeat until close/notification message
        2. Handle agency completion notifications and conduct lottery when all agencies complete
        3. Handle winner requests and return agency-specific winners
        Returns True if successful, False otherwise
        """
        try:
            while True:
                if server and server.shutdown_requested:
                    logging.debug("action: process_communication | result: in_progress |shutdown_requested")
                    return True
                
                # Receive message type first
                message_type = receive_message_type(connection, server)
                
                if message_type == MESSAGE_TYPE_BET:
                    # Receive the complete batch bet message using protocol function
                    # Pass the message_type since we already read it
                    bet_message_data = receive_batch_bet_message_from_connection(connection, message_type, server)
                    
                    # Process the batch bet message and get ACK response
                    ack_data = self.process_batch_bet_message(bet_message_data)
                    
                    connection.send(ack_data)
                    
                elif message_type == MESSAGE_TYPE_NOTIFICATION:
                    # Agency completed sending all bets
                    agency_number = connection.receive_exact_bytes(1)[0]
                    self.process_agency_completion(agency_number)
                    
                    # Send notification response
                    response_data = create_notification_response()
                    connection.send(response_data)
                    
                elif message_type == MESSAGE_TYPE_WINNERS_REQUEST:
                    # Agency requesting winners
                    agency_number = receive_winners_request(connection)
                    
                    if self.is_lottery_completed():
                        # Get winners for this agency
                        winners = self.get_agency_winners(agency_number)
                        response_data = create_winners_available_response(winners)
                    else:
                        # Lottery not ready yet
                        response_data = create_winners_not_available_response()
                    
                    connection.send(response_data)
                    
                elif message_type == MESSAGE_TYPE_CLOSE:
                    return True
                    
                else:
                    logging.error(f"action: unexpected_message_type | result: fail | type: {message_type}")
                    return False
            
        except ShutdownRequestedError as e:
            logging.debug(f"action: process_communication_shutdown | result: in_progress | info: {e}")
            return True
        except ConnectionError as e:
            logging.error(f"action: process_communication | result: fail | error: {e}")
            return False
        except Exception as e:
            logging.error(f"action: process_communication | result: fail | error: {e}")
            return False
