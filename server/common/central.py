import logging
from .utils import Bet, store_bets
from .transport import TCPConnection
from .protocol import (MESSAGE_TYPE_CLOSE,
    deserialize_bet_message, create_ack_message, receive_message_type,
    receive_bet_message_from_connection, send_ack_message_to_connection
)

def process_bet_message(data: bytes):
    """
    Processes a bet message and returns the ACK response
    """
    try:
        # Deserialize the incoming bet message
        bet = deserialize_bet_message(data)
        
        logging.info(f"action: bet_received | result: success | agency: {bet.agency} | "
                    f"name: {bet.first_name} | surname: {bet.last_name} | dni: {bet.document} | "
                    f"birthday: {bet.birthdate} | bet_number: {bet.number}")
        
        # Store the bet using the utils function
        store_bets([bet])
        logging.info(f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}")

        # Create ACK message
        ack_message = create_ack_message(bet)
        
        # Serialize ACK message
        ack_data = ack_message.serialize_ack_bet_message()
        
        logging.info(f"action: ack_created | result: success | agency: {bet.agency} | "
                    f"dni: {bet.document} | bet_number: {bet.number}")
        
        return ack_data
        
    except Exception as e:
        logging.error(f"action: process_bet_message | result: fail | error: {e}")
        raise

def process_communication(connection: TCPConnection):
    """
    Handles the communication protocol:
    1. Receive complete message using protocol functions
    2. Handle message based on type (bet, ack, or close)
    Returns True if successful, False otherwise
    """
    try:
        # Use the protocol function to receive the bet message directly
        # This function will handle reading the message type and all data
        bet_message_data = receive_bet_message_from_connection(connection)
        
        # Process the bet message and get ACK response
        ack_data = process_bet_message(bet_message_data)
        
        # Send ACK response
        send_ack_message_to_connection(connection, ack_data)
        
        # Wait for close message from client
        close_message_type = receive_message_type(connection)
        if close_message_type == MESSAGE_TYPE_CLOSE:
            logging.info("action: close_message_received | result: success")
            return True
        else:
            logging.error(f"action: expected_close_message | result: fail | received_type: {close_message_type}")
            return False
        
    except Exception as e:
        logging.error(f"action: process_communication | result: fail | error: {e}")
        return False
