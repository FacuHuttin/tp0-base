
class Scheduler:
  def __init__(self):
    self.agencies = []
    self.current_index = 0

  def amount_of_agencies(self):
    return len(self.agencies)

  def get_next_agency(self):
    if len(self.agencies) == 0:
      return None
    
    start_index = self.current_index
    while True:
      agency = self.agencies[self.current_index]
      self.current_index = (self.current_index + 1) % len(self.agencies)
      if not agency.bets_stored:
        return agency
      if self.current_index == start_index:
        return None

  def add_agency(self, socket):
    agency = Agency(socket)
    self.agencies.append(agency)
    self.current_index = len(self.agencies) - 1

  def pop_agency(self):
    if not self.agencies:
      return None
    
    popped_agency = self.agencies.pop()
    
    if self.current_index >= len(self.agencies):
      self.current_index = 0
    
    return popped_agency

  def all_bets_stored(self):
    for agency in self.agencies:
      if not agency.bets_stored:
        return False
    return True

class Agency:
  def __init__(self, socket):
    self.socket = socket
    self.winners = []
    self.bets_stored = False
    self.id = None
