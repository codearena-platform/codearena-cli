from codearena_sdk import BaseBot

class MyBot(BaseBot):
    def run(self, state):
        self.ahead(10)
        self.turn(5)
        self.fire(1.0)

    def on_scanned_robot(self, event):
        self.fire(3.0)
