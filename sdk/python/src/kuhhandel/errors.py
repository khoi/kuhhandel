class KuhhandelError(Exception):
    pass


class ClientStateError(KuhhandelError):
    pass


class ConnectionLost(KuhhandelError):
    pass


class RequestTimeout(ConnectionLost):
    pass


class ProtocolError(ConnectionLost):
    pass


class ServerError(KuhhandelError):
    def __init__(self, code: str, message: str, request_id: str) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.request_id = request_id

    def __str__(self) -> str:
        return f"{self.code}: {self.message}"
