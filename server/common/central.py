import logging

from .utils import Bet, store_bets
from .transport import TCPConnection
from .protocol import (
    MESSAGE_TYPE_BET, MESSAGE_TYPE_ACK, MESSAGE_TYPE_CLOSE,
    AGENCY_NUMBER_SIZE, DNI_SIZE, BIRTHDAY_SIZE, BET_NUMBER_SIZE,
    deserialize_batch_bet_message, create_batch_ack_message, receive_message_type,
    receive_batch_bet_message_from_connection, send_ack_message_to_connection
)

def process_batch_bet_message(data: bytes):
    """
    Processes a batch bet message and returns the ACK response
    """
    try:
        # Deserialize the incoming batch bet message
        batch_number, bets = deserialize_batch_bet_message(data)
        
        # Store all bets in the batch
        store_bets(bets)

        # Create ACK message for the batch
        agency_number = bets[0].agency if bets else 0  # Get agency from first bet
        ack_message = create_batch_ack_message(agency_number, batch_number)
        
        # Serialize ACK message
        ack_data = ack_message.serialize_ack_bet_message()
        
        logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
        
        return ack_data
        
    except Exception as e:
        logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)} | error: {e}")
        raise

def process_communication(connection: TCPConnection):
    """
    Handles the communication protocol for batch betting:
    1. Receive batch bet message -> send ACK -> repeat until close message
    2. Each batch is processed independently
    Returns True if successful, False otherwise
    """
    try:
        while True:
            # Receive message type first
            message_type = receive_message_type(connection)
            
            if message_type == MESSAGE_TYPE_BET:
                # Receive the complete batch bet message using protocol function
                # Pass the message_type since we already read it
                bet_message_data = receive_batch_bet_message_from_connection(connection, message_type)
                
                # Process the batch bet message and get ACK response
                ack_data = process_batch_bet_message(bet_message_data)
                
                # Send ACK response
                send_ack_message_to_connection(connection, ack_data)
                
            elif message_type == MESSAGE_TYPE_CLOSE:
                return True
                
            else:
                logging.error(f"action: unexpected_message_type | result: fail | type: {message_type}")
                return False
        
    except Exception as e:
        logging.error(f"action: process_communication | result: fail | error: {e}")
        return False
