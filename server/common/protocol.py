"""
Betting System Protocol Module

This module handles message serialization, deserialization, and protocol constants
for communication between client and server in the betting system.

Protocol Overview:
- All messages start with a message type byte (1 byte)
- Supports batch processing for efficiency
- Fixed fields (DNI, Birthday) have predefined sizes and no length prefix
- Variable fields (Name, Surname) have a length byte followed by content
- BetNumber is a fixed 4-byte field stored as big endian 32-bit integer
- Maximum variable field size is 255 bytes

Message Types:
- MESSAGE_TYPE_BET (1): Client sends bet information to server (supports batches)
- MESSAGE_TYPE_ACK (2): Server acknowledges batch receipt
- MESSAGE_TYPE_CLOSE (3): Graceful connection shutdown
- MESSAGE_TYPE_NOTIFICATION (4): Client notifies completion of bet sending
- MESSAGE_TYPE_NOTIFICATION_ACK (5): Server acknowledges notification
- MESSAGE_TYPE_WINNERS_REQUEST (6): Client requests winners
- MESSAGE_TYPE_WINNERS_NOT_AVAILABLE (7): Server responds winners not ready
- MESSAGE_TYPE_WINNERS_AVAILABLE (8): Server responds with winners

Batch Bet Message Format:
1st byte: message type (MESSAGE_TYPE_BET)
2nd byte: agency number (0-255)
3rd-6th bytes: batch number (4-byte big endian integer)
7th byte: number of bets in batch (n, max 255)
Following bytes: n times bet data where each bet contains:
  - Name: [length_byte][utf-8_content]
  - Surname: [length_byte][utf-8_content]
  - DNI: [8_fixed_bytes]
  - Birthday: [10_fixed_bytes]
  - BetNumber: [4_bytes_big_endian_integer]

ACK Message Format:
1st byte: message type (MESSAGE_TYPE_ACK)
2nd byte: agency number (0-255)
3rd-6th bytes: batch number (4-byte big endian integer)

Winners Response Format:
1st byte: MESSAGE_TYPE_WINNERS_AVAILABLE
2nd byte: number of winners (0-255)
Following bytes: winner DNIs (8 bytes each, fixed length)
"""

import logging
from .utils import Bet
from .transport import TCPConnection, ShutdownRequestedError

MAX_BYTE_VALUE = 255
MAX_4BYTE_VALUE = 0xFFFFFFFF

# Protocol message types (matching the client constants)
MESSAGE_TYPE_BET = 1
MESSAGE_TYPE_ACK = 2
MESSAGE_TYPE_CLOSE = 3
MESSAGE_TYPE_NOTIFICATION = 4
MESSAGE_TYPE_NOTIFICATION_ACK = 5
MESSAGE_TYPE_WINNERS_REQUEST = 6
MESSAGE_TYPE_WINNERS_NOT_AVAILABLE = 7
MESSAGE_TYPE_WINNERS_AVAILABLE = 8

# Common protocol field sizes
MESSAGE_TYPE_SIZE = 1
LENGTH_FIELD_SIZE = 1
AGENCY_NUMBER_SIZE = 1
BATCH_NUMBER_SIZE = 4
BETS_COUNT_SIZE = 1
DNI_SIZE = 8
BIRTHDAY_SIZE = 10
BET_NUMBER_SIZE = 4
MAX_VARIABLE_FIELD_SIZE = 255

# Error message constants
ERR_INSUFFICIENT_DATA = "insufficient data"
ERR_INVALID_BET_NUMBER = "invalid bet number: cannot be empty"
ERR_EMPTY_BET_NUMBER = "invalid BetNumber field: cannot be empty"

class AckBetMessage:
    """
    Represents an acknowledgment message for a batch of bets.
    
    Used to confirm successful receipt of a batch of bets from a specific agency.
    Contains the agency number and batch number for identification.
    
    Attributes:
        agency_number (int): The agency that sent the batch (0 - 255)
        batch_number (int): The batch identifier (0 - 4.294.967.295)
    """

    def __init__(self, agency_number: int, batch_number: int):
        self.agency_number = agency_number
        self.batch_number = batch_number

    def serialize_ack_bet_message(self):
        """
        Serializes the ACK bet message according to the protocol.
        
        Protocol format:
        - 1st byte: message type (MESSAGE_TYPE_ACK)
        - 2nd byte: agency number (0 - 255)
        - 3rd-6th bytes: batch number (4-byte big endian integer)
        
        Returns:
            bytes: Serialized acknowledgment message (6 bytes total)
            
        Raises:
            ValueError: If agency number or batch number is out of valid range
        """        
        result = bytearray()
        
        # Add message type as first byte
        result.append(MESSAGE_TYPE_ACK)
        
        # Add agency number as second byte (validate range)
        if 0 <= self.agency_number <= MAX_BYTE_VALUE:
            result.append(self.agency_number)
        else:
            raise ValueError(f"Agency number out of range: {self.agency_number} (must be 0-255)")
        
        # Add batch number as 4-byte big endian integer
        if 0 <= self.batch_number <= MAX_4BYTE_VALUE:
            batch_bytes = self.batch_number.to_bytes(4, byteorder='big')
            result.extend(batch_bytes)
        else:
            raise ValueError(f"Batch number out of range: {self.batch_number} (must be 0 to {MAX_4BYTE_VALUE})")

        return bytes(result)

def serialize_fixed_field(content: str, expected_size: int, field_name: str) -> bytes:
    """
    Serializes a fixed length field and validates its size.
    
    Args:
        content (str): The string content to serialize
        expected_size (int): Expected byte length after UTF-8 encoding
        field_name (str): Field name for error messages
        
    Returns:
        bytes: Fixed-length field content
        
    Raises:
        ValueError: If encoded content size doesn't match expected size
    """
    content_bytes = content.encode('utf-8')
    if len(content_bytes) != expected_size:
        raise ValueError(f"invalid {field_name} field: must be {expected_size} characters")
    return content_bytes


def validate_message_type(data: bytes, expected_type: int) -> None:
    """
    Validates the message type byte.
    
    Args:
        data (bytes): Message data to validate
        expected_type (int): Expected message type value
        
    Raises:
        ValueError: If data is empty or message type doesn't match
    """
    if len(data) == 0:
        raise ValueError("empty data")
    if data[0] != expected_type:
        raise ValueError(f"invalid message type: expected {expected_type}, got {data[0]}")


# def deserialize_bet_message(data: bytes) -> Bet:
#     """
#     Deserializes a single bet message according to the protocol.

#     Protocol format:
#     - 1st byte: message type (MESSAGE_TYPE_BET)
#     - 2nd byte: agency number
#     - Variable length fields (Name, Surname): [length_byte][content]
#     - Fixed length fields (DNI=8 chars, Birthday=10 chars): [content]
#     - BetNumber: 4-byte big endian unsigned integer
    
#     Args:
#         data (bytes): Raw message data to deserialize
        
#     Returns:
#         Bet: Parsed bet object from utils.py
        
#     Raises:
#         ValueError: If data is malformed or too short
#     """
#     if len(data) < 2:
#         raise ValueError("data too short: missing message type and agency number")
    
#     pos = 0
    
#     # Check message type
#     validate_message_type(data, MESSAGE_TYPE_BET)
#     pos += MESSAGE_TYPE_SIZE
    
#     # Extract agency number (second byte)
#     agency_number = data[pos]
#     pos += AGENCY_NUMBER_SIZE
    
#     # Deserialize Name field: length byte + content
#     if pos >= len(data):
#         raise ValueError("data too short: missing length for Name field")
#     name_length = data[pos]
#     pos += LENGTH_FIELD_SIZE
#     if pos + name_length > len(data):
#         raise ValueError("data too short: missing content for Name field")
#     first_name = data[pos:pos + name_length].decode('utf-8')
#     pos += name_length
    
#     # Deserialize Surname field: length byte + content
#     if pos >= len(data):
#         raise ValueError("data too short: missing length for Surname field")
#     surname_length = data[pos]
#     pos += LENGTH_FIELD_SIZE
#     if pos + surname_length > len(data):
#         raise ValueError("data too short: missing content for Surname field")
#     last_name = data[pos:pos + surname_length].decode('utf-8')
#     pos += surname_length
    
#     # Deserialize DNI field: fixed characters, no length byte
#     if pos + DNI_SIZE > len(data):
#         raise ValueError(f"data too short: missing DNI field ({DNI_SIZE} chars)")
#     document = data[pos:pos + DNI_SIZE].decode('utf-8').rstrip()
#     pos += DNI_SIZE
    
#     # Deserialize Birthday field: fixed characters, no length byte
#     if pos + BIRTHDAY_SIZE > len(data):
#         raise ValueError(f"data too short: missing Birthday field ({BIRTHDAY_SIZE} chars)")
#     birthdate = data[pos:pos + BIRTHDAY_SIZE].decode('utf-8').rstrip()
#     pos += BIRTHDAY_SIZE
    
#     # Deserialize BetNumber field: fixed 4-byte big endian integer
#     if pos + BET_NUMBER_SIZE > len(data):
#         raise ValueError(f"data too short: missing BetNumber field ({BET_NUMBER_SIZE} bytes)")
#     bet_number_bytes = data[pos:pos + BET_NUMBER_SIZE]
#     number = deserialize_bet_number_field(bet_number_bytes)
#     pos += BET_NUMBER_SIZE
    
#     return Bet(
#         agency=agency_number,
#         first_name=first_name,
#         last_name=last_name,
#         document=document,
#         birthdate=birthdate,
#         number=number
#     )


def create_batch_ack_message(agency_number: int, batch_number: int) -> AckBetMessage:
    """
    Creates an ACK message for a batch of bets.
    
    Args:
        agency_number (int): Agency that sent the batch (0 - 255)
        batch_number (int): Batch identifier (0 - 4.294.967.295)
        
    Returns:
        AckBetMessage: Acknowledgment message object
    """
    return AckBetMessage(
        agency_number=agency_number,
        batch_number=batch_number
    )


def receive_message_type(connection: TCPConnection, server=None) -> int:
    """
    Receives and returns the message type from the connection.
    
    Args:
        connection (TCPConnection): Active connection to read from
        server: Server instance for shutdown checking (optional)
        
    Returns:
        int: Message type value (1-8)
        
    Raises:
        ShutdownRequestedError: If shutdown is requested during operation
        ConnectionError: If connection fails or is closed
    """
    try:
        if server and server.shutdown_requested:
            raise ShutdownRequestedError("Shutdown requested")
            
        message_type_data = connection.receive_exact_bytes(MESSAGE_TYPE_SIZE)
        return message_type_data[0]
    except ShutdownRequestedError as e:
        logging.info(f"action: receive_message_type | result: shutdown | info: {e}")
        raise
    except ConnectionError as e:
        logging.error(f"action: receive_message_type | result: fail | error: {e}")
        raise
    except Exception as e:
        logging.error(f"action: receive_message_type | result: fail | error: {e}")
        raise

# def serialize_bet_number_field(bet_number: str) -> bytes:
#     """
#     Converts a bet number string to 4-byte big endian format.
    
#     Args:
#         bet_number (str): Bet number as string
        
#     Returns:
#         bytes: 4-byte big endian representation
        
#     Raises:
#         ValueError: If bet number is invalid or out of 32-bit range
#     """
#     try:
#         bet_int = int(bet_number)
#         if bet_int < 0 or bet_int > MAX_4BYTE_VALUE:
#             raise ValueError(f"bet number {bet_number} is out of range for 32-bit integer")
#         return bet_int.to_bytes(4, byteorder='big')
#     except ValueError as e:
#         raise ValueError(f"invalid bet number: {bet_number} is not a valid integer") from e


def deserialize_bet_number_field(bet_bytes: bytes) -> str:
    """
    Converts 4-byte big endian format to bet number string.
    
    Args:
        bet_bytes (bytes): 4-byte big endian integer data
        
    Returns:
        str: Bet number as string
        
    Raises:
        ValueError: If bet_bytes is not exactly 4 bytes
    """
    if len(bet_bytes) != BET_NUMBER_SIZE:
        raise ValueError(f"bet number field must be exactly 4 bytes, got {len(bet_bytes)}")
    
    bet_int = int.from_bytes(bet_bytes, byteorder='big')
    return str(bet_int)


def deserialize_batch_bet_message(data: bytes) -> tuple[int, list[Bet]]:
    """
    Deserializes batch bet message according to the protocol.

    Protocol format:
    - 1st byte: message type (MESSAGE_TYPE_BET)
    - 2nd byte: agency number
    - 3rd-6th bytes: batch number (4-byte big endian integer)
    - 7th byte: number of bets (n, max 255)
    - Following bytes: n bet records, each containing:
      * Name: [length_byte][content]
      * Surname: [length_byte][content] 
      * DNI: [8_fixed_bytes]
      * Birthday: [10_fixed_bytes]
      * BetNumber: [4_bytes_big_endian]
    
    Args:
        data (bytes): Raw batch message data
        
    Returns:
        tuple[int, list[Bet]]: (batch_number, list_of_bet_objects)
        
    Raises:
        ValueError: If data is malformed, too short, or contains invalid fields
    """
    if len(data) < (MESSAGE_TYPE_SIZE + AGENCY_NUMBER_SIZE + BATCH_NUMBER_SIZE + BETS_COUNT_SIZE):
        raise ValueError("data too short: missing message type, agency number, batch number, or bets count")
    
    pos = 0
    
    # Check message type
    validate_message_type(data, MESSAGE_TYPE_BET)
    pos += MESSAGE_TYPE_SIZE
    
    # Extract agency number (second byte)
    agency_number = data[pos]
    pos += AGENCY_NUMBER_SIZE

    # Extract batch number (3rd-6th bytes, big endian)
    if pos + BATCH_NUMBER_SIZE > len(data):
        raise ValueError("data too short: missing batch number")
    batch_number_bytes = data[pos:pos + BATCH_NUMBER_SIZE]
    batch_number = int.from_bytes(batch_number_bytes, byteorder='big')
    pos += BATCH_NUMBER_SIZE
    
    # Extract number of bets (7th byte)
    bets_count = data[pos]
    pos += BETS_COUNT_SIZE

    bets = []
    
    # Deserialize each bet
    for i in range(bets_count):
        # Deserialize Name field: length byte + content
        if pos >= len(data):
            raise ValueError(f"data too short: missing length for Name field of bet {i+1}")
        name_length = data[pos]
        pos += LENGTH_FIELD_SIZE
        if pos + name_length > len(data):
            raise ValueError(f"data too short: missing content for Name field of bet {i+1}")
        first_name = data[pos:pos + name_length].decode('utf-8')
        pos += name_length
        
        # Deserialize Surname field: length byte + content
        if pos >= len(data):
            raise ValueError(f"data too short: missing length for Surname field of bet {i+1}")
        surname_length = data[pos]
        pos += LENGTH_FIELD_SIZE
        if pos + surname_length > len(data):
            raise ValueError(f"data too short: missing content for Surname field of bet {i+1}")
        last_name = data[pos:pos + surname_length].decode('utf-8')
        pos += surname_length
        
        # Deserialize DNI field: fixed characters, no length byte
        if pos + DNI_SIZE > len(data):
            raise ValueError(f"data too short: missing DNI field ({DNI_SIZE} chars) for bet {i+1}")
        document = data[pos:pos + DNI_SIZE].decode('utf-8').rstrip()
        pos += DNI_SIZE
        
        # Deserialize Birthday field: fixed characters, no length byte
        if pos + BIRTHDAY_SIZE > len(data):
            raise ValueError(f"data too short: missing Birthday field ({BIRTHDAY_SIZE} chars) for bet {i+1}")
        birthdate = data[pos:pos + BIRTHDAY_SIZE].decode('utf-8').rstrip()
        pos += BIRTHDAY_SIZE
        
        # Deserialize BetNumber field: fixed 4-byte big endian integer
        if pos + BET_NUMBER_SIZE > len(data):
            raise ValueError(f"data too short: missing BetNumber field ({BET_NUMBER_SIZE} bytes) for bet {i+1}")
        bet_number_bytes = data[pos:pos + BET_NUMBER_SIZE]
        number = deserialize_bet_number_field(bet_number_bytes)
        pos += BET_NUMBER_SIZE
        
        # Create Bet object and add to list
        bet = Bet(
            agency=agency_number,
            first_name=first_name,
            last_name=last_name,
            document=document,
            birthdate=birthdate,
            number=number
        )
        bets.append(bet)
    
    return batch_number, bets


def receive_batch_bet_message_from_connection(connection: TCPConnection, message_type: int = None, server=None) -> bytes:
    """
    Receives a batch bet message from connection using protocol knowledge.
    
    Reads the complete batch message by parsing the protocol structure to determine
    the exact amount of data needed for all bets in the batch.
    
    Args:
        connection (TCPConnection): Active connection to read from
        message_type (int, optional): Pre-read message type. If None, will read it
        server: Server instance for shutdown checking (optional)
        
    Returns:
        bytes: Complete batch message data including the message type header
        
    Raises:
        ShutdownRequestedError: If shutdown is requested during operation
        ValueError: If message type is invalid
        ConnectionError: If connection fails during read
    """
    try:
        if server and server.shutdown_requested:
            raise ShutdownRequestedError("Shutdown requested")
        
        if message_type is None:
            # Read the message type (1 byte)
            message_type_data = connection.receive_exact_bytes(MESSAGE_TYPE_SIZE)
            message_type = message_type_data[0]
            
            # Verify it's a bet message
            if message_type != MESSAGE_TYPE_BET:
                raise ValueError(f"expected bet message type {MESSAGE_TYPE_BET}, got {message_type}")
        else:
            # Message type was already read, create the data
            message_type_data = bytes([message_type])
        
        # Read the agency number (1 byte)
        agency_data = connection.receive_exact_bytes(AGENCY_NUMBER_SIZE)
        
        # Read batch number (1 byte)
        batch_data = connection.receive_exact_bytes(BATCH_NUMBER_SIZE)
        
        # Read number of bets (1 byte)
        bets_count_data = connection.receive_exact_bytes(BETS_COUNT_SIZE)
        bets_count = bets_count_data[0]
        
        # Read the data for all bets
        remaining_data = bytearray()
        
        for _ in range(bets_count):
            if server and server.shutdown_requested:
                raise ShutdownRequestedError("Shutdown requested")
                
            # Read name length and content
            name_length_data = connection.receive_exact_bytes(LENGTH_FIELD_SIZE)
            name_length = name_length_data[0]
            remaining_data.extend(name_length_data)
            if name_length > 0:
                name_data = connection.receive_exact_bytes(name_length)
                remaining_data.extend(name_data)
            
            # Read surname length and content
            surname_length_data = connection.receive_exact_bytes(LENGTH_FIELD_SIZE)
            surname_length = surname_length_data[0]
            remaining_data.extend(surname_length_data)
            if surname_length > 0:
                surname_data = connection.receive_exact_bytes(surname_length)
                remaining_data.extend(surname_data)
            
            # Read fixed fields
            dni_data = connection.receive_exact_bytes(DNI_SIZE)
            birthday_data = connection.receive_exact_bytes(BIRTHDAY_SIZE)
            bet_number_data = connection.receive_exact_bytes(BET_NUMBER_SIZE)
            
            remaining_data.extend(dni_data)
            remaining_data.extend(birthday_data)
            remaining_data.extend(bet_number_data)
        
        # Combine all data according to protocol (including message type)
        full_message = (message_type_data + agency_data + batch_data + bets_count_data + bytes(remaining_data))
        return full_message
        
    except ShutdownRequestedError as e:
        logging.info(f"action: receive_batch_bet_message_from_connection | result: shutdown | info: {e}")
        raise
    except ConnectionError as e:
        logging.error(f"action: receive_batch_bet_message_from_connection | result: fail | error: {e}")
        raise
    except Exception as e:
        logging.error(f"action: receive_batch_bet_message_from_connection | result: fail | error: {e}")
        raise


def create_notification_response() -> bytes:
    """
    Creates a response message for agency completion notification.
    
    Returns:
        bytes: Single-byte acknowledgment message (MESSAGE_TYPE_NOTIFICATION_ACK)
    """
    return bytes([MESSAGE_TYPE_NOTIFICATION_ACK])


def create_winners_not_available_response() -> bytes:
    """
    Creates a response indicating winners are not yet available.
    
    Returns:
        bytes: Single-byte response message (MESSAGE_TYPE_WINNERS_NOT_AVAILABLE)
    """
    return bytes([MESSAGE_TYPE_WINNERS_NOT_AVAILABLE])


def create_winners_available_response(winners: list[str]) -> bytes:
    """
    Creates a response with the list of winning DNIs for an agency.
    
    Protocol format:
    - 1st byte: MESSAGE_TYPE_WINNERS_AVAILABLE
    - 2nd byte: number of winners (0 - 255)
    - Following bytes: winner DNIs (8 bytes each, fixed length, space-padded)
    
    Args:
        winners (list[str]): List of winning DNI strings
        
    Returns:
        bytes: Serialized winners response message
        
    Note:
        Limited to maximum 255 winners. DNIs longer than 8 bytes are truncated,
        shorter ones are space-padded to 8 bytes.
    """
    result = bytearray()
    result.append(MESSAGE_TYPE_WINNERS_AVAILABLE)
    
    # Add number of winners
    num_winners = len(winners)
    if num_winners > MAX_BYTE_VALUE:
        num_winners = MAX_BYTE_VALUE
        winners = winners[:MAX_BYTE_VALUE]

    result.append(num_winners)
    
    # Add each winner DNI (8 bytes each)
    for dni in winners:
        dni_bytes = serialize_fixed_field(dni, DNI_SIZE, "DNI")
        result.extend(dni_bytes)
    
    return bytes(result)


def receive_winners_request(connection: TCPConnection) -> int:
    """
    Receives a winners request message from a client.
    
    Protocol format:
    - [MESSAGE_TYPE_WINNERS_REQUEST][agency_number]
    
    Note: The message type byte should already be consumed before calling this function.
    
    Args:
        connection (TCPConnection): Active connection to read from
        
    Returns:
        int: The agency number requesting winners (0-255)
        
    Raises:
        ConnectionError: If connection fails during read
    """
    agency_data = connection.receive_exact_bytes(AGENCY_NUMBER_SIZE)
    return agency_data[0]
