import type { LineupUpdate } from '../api/types';
import type { SquadResult } from './recommender';
import { sortByPosAndScore } from './recommender';
import { PosGK } from '../api/types';

export function planLineup(result: SquadResult): LineupUpdate {
  const starters = [...result.starters];
  sortByPosAndScore(starters);

  const benchGk = result.bench.filter((sp) => sp.player.element_type === PosGK);
  const benchOutfield = result.bench.filter((sp) => sp.player.element_type !== PosGK);
  const orderedBench = [...benchGk, ...benchOutfield];

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
      is_captain: isC,
      is_vice_captain: isVc,
    });
    pos++;
  });

  orderedBench.forEach((sp) => {
    picks.push({
      element: sp.player.id,
      position: pos,
      is_captain: false,
      is_vice_captain: false,
    });
    pos++;
  });

  return { picks, chip: null };
}
