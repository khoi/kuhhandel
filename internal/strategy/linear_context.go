package strategy

import "github.com/khoi/kuhhandel/internal/game"

func linearInteractions(decision int, features linearFeatures) [8]float64 {
	switch decision {
	case linearTurnDecision:
		return [8]float64{
			features[1] * features[6],
			features[1] * features[7],
			features[3] * features[6],
			features[4] * features[7],
			features[11] * features[6],
			features[12] * features[8],
			features[12] * features[9],
			features[2] * features[1],
		}
	case linearBidDecision, linearResolveDecision:
		return [8]float64{
			features[1] * features[1],
			features[5] * features[5],
			features[6] * features[6],
			features[1] * features[5],
			features[1] * features[6],
			features[11] * features[5],
			features[11] * features[6],
			features[9] * features[10],
		}
	default:
		return [8]float64{
			features[1] * features[1],
			features[2] * features[2],
			features[1] * features[7],
			features[1] * features[8],
			features[11] * features[7],
			features[11] * features[8],
			features[9] * features[10],
			features[12] * features[1],
		}
	}
}

func linearOtherPlayer(decision int, snapshot game.Snapshot, action any) string {
	switch decision {
	case linearTurnDecision:
		if trade, ok := action.(game.BeginTrade); ok {
			return trade.TargetID
		}
	case linearBidDecision, linearResolveDecision:
		if snapshot.Public.Auction != nil {
			return snapshot.Public.Auction.HighestBidderID
		}
	default:
		if snapshot.Public.Trade != nil {
			if snapshot.Public.Trade.ChallengerID == snapshot.Self.PlayerID {
				return snapshot.Public.Trade.TargetID
			}
			return snapshot.Public.Trade.ChallengerID
		}
	}
	return ""
}
