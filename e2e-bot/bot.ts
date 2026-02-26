import { BaseBot, MatchState } from '@codearena/sdk';

export class MyBot extends BaseBot {
    public run(state: MatchState): void {
        this.ahead(10);
        this.turn(5);
        this.fire(1.0);
    }

    public onScannedRobot(event: any): void {
        this.fire(3.0);
    }
}
