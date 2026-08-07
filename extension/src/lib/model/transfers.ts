import type { MyTeam, Pick } from '../api/types';
import type { ScoredPlayer } from './scorer';
import type { SquadResult } from './recommender';

export interface TransferSuggestion {
  out: ScoredPlayer;
  in: ScoredPlayer;
  sellingPrice: number; // tenths of £m
  purchasePrice: number; // tenths of £m
  scoreUplift: number;
}

export function planTransfers(
  current: MyTeam | null,
  optimal: SquadResult,
  maxHits = 4,
  idToTeam: Map<number, number>
): TransferSuggestion[] {
  if (!current || !current.picks || current.picks.length === 0) {
    return [];
  }

  const currentById = new Map<number, Pick>();
  current.picks.forEach((p) => currentById.set(p.element, p));

  const optimalById = new Map<number, ScoredPlayer>();
  optimal.starters.forEach((p) => optimalById.set(p.player.id, p));
  optimal.bench.forEach((p) => optimalById.set(p.player.id, p));

  const incoming: ScoredPlayer[] = [];
  optimalById.forEach((sp, id) => {
    if (!currentById.has(id)) {
      incoming.push(sp);
    }
  });
  incoming.sort((a, b) => b.score - a.score);

  interface OutCand {
    pick: Pick;
    elementType: number;
  }
  const outgoing: OutCand[] = [];
  currentById.forEach((pick, id) => {
    if (!optimalById.has(id)) {
      outgoing.push({
        pick,
        elementType: pick.element_type || 1,
      });
    }
  });
  outgoing.sort((a, b) => (a.pick.selling_price || 0) - (b.pick.selling_price || 0));

  let bank = current.transfers?.bank || 0;
  const usedFree = current.transfers?.made || 0;
  const teamCounts = getTeamCounts(current.picks, idToTeam);

  const limit = current.transfers?.limit;
  const unlimited = current.transfers?.status === 'unlimited';
  const freeLimit = unlimited ? 1000 : limit !== null && limit !== undefined ? limit : 0;

  const suggestions: TransferSuggestion[] = [];
  const used = new Array(outgoing.length).fill(false);

  const freeRemaining = Math.max(0, freeLimit - usedFree);
  const maxByHits = unlimited ? 30 : Math.max(0, freeRemaining + Math.floor(maxHits / 4));

  for (const inPlayer of incoming) {
    if (suggestions.length >= maxByHits) break;

    const inTeam = idToTeam.get(inPlayer.player.id);
    const inTeamKnown = inTeam !== undefined;

    let bestIdx = -1;
    let bestUplift = -1e18;

    for (let i = 0; i < outgoing.length; i++) {
      if (used[i]) continue;

      const outPick = outgoing[i].pick;
      if (outgoing[i].elementType !== inPlayer.player.element_type) continue;

      const sellingPrice = outPick.selling_price || outPick.purchase_price || inPlayer.player.now_cost;
      const netCost = inPlayer.player.now_cost - sellingPrice;
      if (netCost > bank) continue;

      const uplift = inPlayer.score;
      if (uplift > bestUplift) {
        bestUplift = uplift;
        bestIdx = i;
      }
    }

    if (bestIdx < 0) continue;

    const outPick = outgoing[bestIdx].pick;
    const sellingPrice = outPick.selling_price || outPick.purchase_price || inPlayer.player.now_cost;

    if (inTeamKnown && inTeam !== undefined) {
      let postInCount = teamCounts.get(inTeam) || 0;
      const outTeam = idToTeam.get(outPick.element);
      if (outTeam === inTeam) {
        postInCount--;
      }
      if (postInCount >= 3) continue;
    }

    const outSP: ScoredPlayer = {
      player: {
        id: outPick.element,
        element_type: outPick.element_type || 1,
        first_name: '',
        second_name: '',
        web_name: `Player #${outPick.element}`,
        team: idToTeam.get(outPick.element) || 0,
        team_code: 0,
        now_cost: sellingPrice,
        cost_change_start: 0,
        cost_change_event: 0,
        total_points: 0,
        event_points: 0,
        points_per_game: '0',
        form: '0',
        ep_next: null,
        ep_this: null,
        selected_by_percent: '0',
        status: 'a',
        news: '',
        news_added: null,
        minutes: 0,
        goals_scored: 0,
        assists: 0,
        clean_sheets: 0,
        goals_conceded: 0,
        own_goals: 0,
        penalties_saved: 0,
        penalties_missed: 0,
        yellow_cards: 0,
        red_cards: 0,
        saves: 0,
        bonus: 0,
        bps: 0,
        influence: '0',
        creativity: '0',
        threat: '0',
        ict_index: '0',
        starts: 0,
        expected_goals: '0',
        expected_assists: '0',
        expected_goal_involvements: '0',
        expected_goals_conceded: '0',
        chance_of_playing_next_round: null,
      },
      score: 0,
      teamName: '???',
      positionName: '',
      epNextVal: 0,
      formVal: 0,
      ppgVal: 0,
      xgiP90: 0,
      ictP90: 0,
      oppScore: 0,
      oppDesc: '',
      hasFixture: true,
      isHome: false,
      isDgw: false,
      fdrVal: 0,
      upcomingFixtures: '',
      valueRating: 0,
      ownershipPct: 0,
    };

    suggestions.push({
      out: outSP,
      in: inPlayer,
      sellingPrice,
      purchasePrice: inPlayer.player.now_cost,
      scoreUplift: bestUplift,
    });

    bank -= inPlayer.player.now_cost - sellingPrice;
    used[bestIdx] = true;

    const outTeam = idToTeam.get(outPick.element);
    if (outTeam !== undefined) {
      teamCounts.set(outTeam, Math.max(0, (teamCounts.get(outTeam) || 0) - 1));
    }
    if (inTeam !== undefined) {
      teamCounts.set(inTeam, (teamCounts.get(inTeam) || 0) + 1);
    }
  }

  return suggestions;
}

function getTeamCounts(picks: Pick[], idToTeam: Map<number, number>): Map<number, number> {
  const counts = new Map<number, number>();
  picks.forEach((p) => {
    const t = idToTeam.get(p.element);
    if (t !== undefined) {
      counts.set(t, (counts.get(t) || 0) + 1);
    }
  });
  return counts;
}
