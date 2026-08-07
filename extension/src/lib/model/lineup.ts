import type { LineupUpdate } from '../api/types';
import type { SquadResult } from './recommender';
import { sortByPosAndScore } from './recommender';

export function planLineup(result: SquadResult): LineupUpdate {
  const starters = [...result.starters];
  sortByPosAndScore(starters);

  const bench = [...result.bench];

  const captainId = result.captain?.player.id || 0;
  const viceCaptainId = result.viceCaptain?.player.id || 0;

  const picks: LineupUpdate['picks'] = [];

  let pos = 1;
  starters.forEach((sp) => {
    const isC = sp.player.id === captainId;
    const isVc = sp.player.id === viceCaptainId;
    picks.push({
      element: sp.player.id,
      position: pos,
      multiplier: isC ? 2 : 1,
      is_captain: isC,
      is_vice_captain: isVc,
    });
    pos++;
  });

  bench.forEach((sp) => {
    picks.push({
      element: sp.player.id,
      position: pos,
      multiplier: 0,
      is_captain: false,
      is_vice_captain: false,
    });
    pos++;
  });

  return { picks };
}
