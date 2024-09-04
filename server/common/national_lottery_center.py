from .utils import store_bets

class NationalLotteryCenter:
  def __init__(self, server_port, server_listen_backlog):
    from .server import Server
    self.server = Server(server_port, server_listen_backlog, self)

  def start(self):
    self.server.run()

  def store_bets_from_agency(self, bets):
    store_bets(bets)