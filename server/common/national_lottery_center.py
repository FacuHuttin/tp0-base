from .utils import store_bets, load_bets, has_won

class NationalLotteryCenter:
  def __init__(self, server_port, server_listen_backlog, max_amount_of_agencies):
    from .server import Server
    self.server = Server(server_port, server_listen_backlog, max_amount_of_agencies, self)

  def start(self):
    self.server.run()

  def store_bets_from_agency(self, bets):
    store_bets(bets)

  def get_winners(self):
    bets = load_bets()
    winning_bets = [bet for bet in bets if has_won(bet)]
    
    winners_by_agency = {}
    for bet in winning_bets:
      if bet.agency not in winners_by_agency:
        winners_by_agency[bet.agency] = []
      winners_by_agency[bet.agency].append(bet.document)

    return winners_by_agency