"""
Protocol module for the betting system server.

This module handles message serialization, deserialization, and protocol constants
for communication between client and server.

Protocol Overview:
- All messages start with a message type byte
- Fixed fields (DNI, Birthday) have predefined sizes and no length prefix
- Variable fields (Name, Surname) have a length byte followed by content
- BetNumber is a fixed 4-byte field stored as big endian 32-bit integer
- Maximum variable field size is 255 bytes

Message Types:
- MESSAGE_TYPE_BET (1): Client sends bet information to server
- MESSAGE_TYPE_ACK (2): Server acknowledges bet receipt
- MESSAGE_TYPE_CLOSE (3): Graceful connection shutdown
"""

import logging
from .utils import Bet
from .transport import TCPConnection

# Protocol message types (matching the client constants)
MESSAGE_TYPE_BET = 1
MESSAGE_TYPE_ACK = 2
MESSAGE_TYPE_CLOSE = 3

# Common protocol field sizes
MESSAGE_TYPE_SIZE = 1
AGENCY_NUMBER_SIZE = 1
DNI_SIZE = 8
BIRTHDAY_SIZE = 10
BET_NUMBER_SIZE = 4
MAX_VARIABLE_FIELD_SIZE = 255
MAX_BYTE_NUMBER_SIZE = 255

# Error message constants
ERR_INSUFFICIENT_DATA = "insufficient data"
ERR_INVALID_BET_NUMBER = "invalid bet number: cannot be empty"
ERR_EMPTY_BET_NUMBER = "invalid BetNumber field: cannot be empty"

class AckBetMessage:
    """Represents an acknowledgment message for a bet"""

    def __init__(self, agency_number: str, dni: str, bet_number: str):
        self.agency_number = agency_number
        self.dni = dni
        self.bet_number = bet_number

    def serialize_ack_bet_message(self):
        """
        Serializes the ACK bet message according to the protocol:
        1st byte: message type (MESSAGE_TYPE_ACK)
        2nd byte: agency number
        Following 8 bytes: DNI (padded with spaces if shorter)
        Following 4 bytes: bet number (big endian 32-bit integer)
        """
        if not self.bet_number or len(self.bet_number) == 0:
            raise ValueError(ERR_EMPTY_BET_NUMBER)
        
        result = bytearray()
        
        # Add message type as first byte
        result.append(MESSAGE_TYPE_ACK)
        
        # Add agency number as second byte
        try:
            agency_num = int(self.agency_number)
            if 0 <= agency_num <= MAX_BYTE_NUMBER_SIZE:
                result.append(agency_num)
            else:
                result.append(0)  # Default fallback for out of range
        except ValueError:
            result.append(0)  # Default fallback for invalid number
        
        # Add DNI as fixed 8 bytes
        dni_bytes = serialize_fixed_field(self.dni, DNI_SIZE, "DNI")
        result.extend(dni_bytes)
        
        # Add bet number as 4-byte big endian integer
        bet_number_bytes = serialize_bet_number_field(self.bet_number)
        result.extend(bet_number_bytes)
        
        return bytes(result)

def serialize_variable_field(content: str) -> bytes:
    """Serializes a variable length field with length prefix"""
    content_bytes = content.encode('utf-8')
    if len(content_bytes) > MAX_VARIABLE_FIELD_SIZE:
        content_bytes = content_bytes[:MAX_VARIABLE_FIELD_SIZE]
    
    result = bytearray()
    result.append(len(content_bytes))
    result.extend(content_bytes)
    return bytes(result)

def serialize_fixed_field(content: str, expected_size: int, field_name: str) -> bytes:
    """Serializes a fixed length field and validates its size"""
    content_bytes = content.encode('utf-8')
    if len(content_bytes) != expected_size:
        raise ValueError(f"invalid {field_name} field: must be {expected_size} characters")
    return content_bytes

def validate_message_type(data: bytes, expected_type: int) -> None:
    """Validates the message type byte"""
    if len(data) == 0:
        raise ValueError("empty data")
    if data[0] != expected_type:
        raise ValueError(f"invalid message type: expected {expected_type}, got {data[0]}")

def deserialize_bet_message(data: bytes) -> Bet:
    """
    Deserializes bet message according to the protocol:

    1st byte: message type (MESSAGE_TYPE_BET)

    2nd byte: agency number

    Variable length fields (Name, Surname): 1 byte length + string content

    Fixed length fields (DNI=8 chars, Birthday=10 chars): direct content

    BetNumber: 4-byte big endian unsigned integer
    
    Returns a Bet object from utils.py
    """
    if len(data) < 2:
        raise ValueError("data too short: missing message type and agency number")
    
    pos = 0
    
    # Check message type
    validate_message_type(data, MESSAGE_TYPE_BET)
    pos += 1
    
    # Extract agency number (second byte)
    agency_number = data[pos]
    pos += 1
    
    # Deserialize Name field: length byte + content
    if pos >= len(data):
        raise ValueError("data too short: missing length for Name field")
    name_length = data[pos]
    pos += 1
    if pos + name_length > len(data):
        raise ValueError("data too short: missing content for Name field")
    first_name = data[pos:pos + name_length].decode('utf-8')
    pos += name_length
    
    # Deserialize Surname field: length byte + content
    if pos >= len(data):
        raise ValueError("data too short: missing length for Surname field")
    surname_length = data[pos]
    pos += 1
    if pos + surname_length > len(data):
        raise ValueError("data too short: missing content for Surname field")
    last_name = data[pos:pos + surname_length].decode('utf-8')
    pos += surname_length
    
    # Deserialize DNI field: fixed characters, no length byte
    if pos + DNI_SIZE > len(data):
        raise ValueError(f"data too short: missing DNI field ({DNI_SIZE} chars)")
    document = data[pos:pos + DNI_SIZE].decode('utf-8').rstrip()
    pos += DNI_SIZE
    
    # Deserialize Birthday field: fixed characters, no length byte
    if pos + BIRTHDAY_SIZE > len(data):
        raise ValueError(f"data too short: missing Birthday field ({BIRTHDAY_SIZE} chars)")
    birthdate = data[pos:pos + BIRTHDAY_SIZE].decode('utf-8').rstrip()
    pos += BIRTHDAY_SIZE
    
    # Deserialize BetNumber field: fixed 4-byte big endian integer
    if pos + BET_NUMBER_SIZE > len(data):
        raise ValueError(f"data too short: missing BetNumber field ({BET_NUMBER_SIZE} bytes)")
    bet_number_bytes = data[pos:pos + BET_NUMBER_SIZE]
    number = deserialize_bet_number_field(bet_number_bytes)
    pos += BET_NUMBER_SIZE
    
    # Create and return Bet object using the constructor from utils.py
    return Bet(
        agency=agency_number,
        first_name=first_name,
        last_name=last_name,
        document=document,
        birthdate=birthdate,
        number=number
    )

def create_ack_message(bet: Bet) -> AckBetMessage:
    """Creates an ACK message from bet information"""
    if not str(bet.number) or len(str(bet.number)) == 0:
        raise ValueError("cannot create ACK for bet with empty bet number")
    
    return AckBetMessage(
        agency_number=str(bet.agency),
        dni=bet.document,
        bet_number=str(bet.number)
    )

def receive_message_type(connection: TCPConnection) -> int:
    """
    Receives and returns the message type from the connection
    """
    try:
        message_type_data = connection.receive_exact_bytes(1)
        return message_type_data[0]
    except Exception as e:
        logging.error(f"action: receive_message_type | result: fail | error: {e}")
        raise

def receive_bet_message_from_connection(connection: TCPConnection) -> bytes:
    """
    Receives a bet message from connection using protocol knowledge
    Returns the complete message data as bytes
    """
    try:
        # Start by reading the message type (1 byte)
        message_type_data = connection.receive_exact_bytes(1)
        message_type = message_type_data[0]
        
        # Verify it's a bet message
        if message_type != MESSAGE_TYPE_BET:
            raise ValueError(f"expected bet message type {MESSAGE_TYPE_BET}, got {message_type}")
        
        # Read the agency number (1 byte)
        agency_data = connection.receive_exact_bytes(AGENCY_NUMBER_SIZE)
        
        # Read name length and name content
        name_length_data = connection.receive_exact_bytes(1)
        name_length = name_length_data[0]
        name_data = connection.receive_exact_bytes(name_length) if name_length > 0 else b''
        
        # Read surname length and surname content
        surname_length_data = connection.receive_exact_bytes(1)
        surname_length = surname_length_data[0]
        surname_data = connection.receive_exact_bytes(surname_length) if surname_length > 0 else b''
        
        # Read fixed DNI
        dni_data = connection.receive_exact_bytes(DNI_SIZE)
        
        # Read fixed Birthday
        birthday_data = connection.receive_exact_bytes(BIRTHDAY_SIZE)
        
        # Read bet number as fixed 4-byte big endian integer
        bet_data = connection.receive_exact_bytes(BET_NUMBER_SIZE)
        
        # Combine all data according to protocol (including message type)
        full_message = (message_type_data + agency_data + 
                       name_length_data + name_data +
                       surname_length_data + surname_data +
                       dni_data + 
                       birthday_data + 
                       bet_data)
        
        logging.info(f"action: bet_message_received | result: success | total_bytes: {len(full_message)}")
        return full_message
        
    except Exception as e:
        logging.error(f"action: receive_bet_message_from_connection | result: fail | error: {e}")
        raise

def send_ack_message_to_connection(connection: TCPConnection, ack_data: bytes) -> None:
    """
    Sends ACK message to connection
    Handles the low-level sending with proper error handling
    """
    try:
        connection.send(ack_data)
        logging.info(f"action: ack_message_sent | result: success | bytes: {len(ack_data)}")
    except Exception as e:
        logging.error(f"action: send_ack_message_to_connection | result: fail | error: {e}")
        raise

def serialize_bet_number_field(bet_number: str) -> bytes:
    """Converts a bet number string to 4-byte big endian format"""
    try:
        bet_int = int(bet_number)
        if bet_int < 0 or bet_int > 0xFFFFFFFF:
            raise ValueError(f"bet number {bet_number} is out of range for 32-bit integer")
        return bet_int.to_bytes(4, byteorder='big')
    except ValueError as e:
        raise ValueError(f"invalid bet number: {bet_number} is not a valid integer") from e

def deserialize_bet_number_field(bet_bytes: bytes) -> str:
    """Converts 4-byte big endian format to bet number string"""
    if len(bet_bytes) != 4:
        raise ValueError(f"bet number field must be exactly 4 bytes, got {len(bet_bytes)}")
    
    bet_int = int.from_bytes(bet_bytes, byteorder='big')
    return str(bet_int)
