import socket
import logging

TIMEOUT = 1.0

class ShutdownRequestedError(ConnectionError):
    """Raised when a shutdown is requested during an operation"""
    pass

class Connection:
    """Abstract connection interface"""
    
    def receive_exact_bytes(self, count):
        raise NotImplementedError
    
    def send(self, data):
        raise NotImplementedError
    
    def close(self):
        raise NotImplementedError

class TCPConnection(Connection):
    """TCP connection implementation for server-side"""
    
    def __init__(self, client_socket: socket.socket, server=None):
        self.client_socket = client_socket
        self.address = client_socket.getpeername() if client_socket else None
        self.server = server  # Reference to server instance for shutdown checking
        # Set a timeout to prevent blocking indefinitely during shutdown
        self.client_socket.settimeout(TIMEOUT)
    
    def receive_exact_bytes(self, count):
        """Receive exactly count bytes from the socket"""
        data = bytearray(count)
        total_read = 0
        
        while total_read < count:
            # Check if shutdown was requested via server instance
            if self.server and self.server.shutdown_requested:
                raise ShutdownRequestedError("Shutdown requested while reading data")
            
            try:
                chunk = self.client_socket.recv(count - total_read)
                if not chunk:
                    # Connection was closed - check if it's due to shutdown
                    if self.server and self.server.shutdown_requested:
                        raise ShutdownRequestedError("Connection closed during shutdown")
                    raise ConnectionError("Connection closed while reading data")
                
                chunk_len = len(chunk)
                data[total_read:total_read + chunk_len] = chunk
                total_read += chunk_len
            except socket.timeout:
                # On timeout, check shutdown flag and continue if not shutting down
                if self.server and self.server.shutdown_requested:
                    raise ShutdownRequestedError("Shutdown requested while reading data")
                continue
            except socket.error as e:
                if self.server and self.server.shutdown_requested:
                    raise ShutdownRequestedError("Shutdown requested while reading data")
                raise ConnectionError(f"Socket error while reading data: {e}")
        
        return bytes(data)
    
    def send(self, data: bytes):
        """Send data with short-write protection"""
        
        total_sent = 0
        data_len = len(data)
        
        while total_sent < data_len:
            try:
                sent = self.client_socket.send(data[total_sent:])
                if sent == 0:
                    raise ConnectionError("Socket connection broken")
                total_sent += sent
            except socket.error as e:
                logging.error(f"action: send | result: fail | error: {e}")
                raise
    
    def close(self):
        """Close the connection"""
        try:
            if self.client_socket:
                self.client_socket.close()
        except socket.error as e:
            logging.error(f"action: close_connection | result: fail | error: {e}")
    
    def get_address(self):
        """Get client address"""
        return self.address
