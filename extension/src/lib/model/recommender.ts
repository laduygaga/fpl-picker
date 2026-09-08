import {
  PosGK,
  PosDEF,
  PosMID,
  PosFWD,
} from '../api/types';
import type { ScoredPlayer, FixturePairing } from './scorer';

export interface Formation {
  name: string;
  gk: number;
  def: number;
  mid: number;
  fwd: number;
}

export const VALID_FORMATIONS: Formation[] = [
  { name: '3-4-3', gk: 1, def: 3, mid: 4, fwd: 3 },
  { name: '3-5-2', gk: 1, def: 3, mid: 5, fwd: 2 },
  { name: '4-3-3', gk: 1, def: 4, mid: 3, fwd: 3 },
  { name: '4-4-2', gk: 1, def: 4, mid: 4, fwd: 2 },
  { name: '4-5-1', gk: 1, def: 4, mid: 5, fwd: 1 },
  { name: '5-3-2', gk: 1, def: 5, mid: 3, fwd: 2 },
  { name: '5-4-1', gk: 1, def: 5, mid: 4, fwd: 1 },
];

export const SQUAD_SLOTS: Record<number, number> = {
  [PosGK]: 2,
  [PosDEF]: 5,
  [PosMID]: 5,
  [PosFWD]: 3,
};

export interface SquadResult {
  formation: string;
  starters: ScoredPlayer[];
  bench: ScoredPlayer[];
  captain: ScoredPlayer | null;
  viceCaptain: ScoredPlayer | null;
  totalScore: number;
  xiCost: number; // £M
  totalCost: number; // £M
  budget: number; // £M
}

class PosCounts {
  readonly sGK: number;
  readonly sDEF: number;
  readonly sMID: number;
  readonly sFWD: number;

  constructor(sGK: number, sDEF: number, sMID: number, sFWD: number) {
    this.sGK = sGK;
    this.sDEF = sDEF;
    this.sMID = sMID;
    this.sFWD = sFWD;
  }

  get key(): string {
    return `${this.sGK},${this.sDEF},${this.sMID},${this.sFWD}`;
  }

  add(other: PosCounts): PosCounts {
    return new PosCounts(
      this.sGK + other.sGK,
      this.sDEF + other.sDEF,
      this.sMID + other.sMID,
      this.sFWD + other.sFWD
    );
  }

  fitsWithin(target: PosCounts): boolean {
    return (
      this.sGK <= target.sGK &&
      this.sDEF <= target.sDEF &&
      this.sMID <= target.sMID &&
      this.sFWD <= target.sFWD
    );
  }
}

interface TeamOption {
  counts: PosCounts;
  cost: number; // tenths of £m
  score: number;
  players: ScoredPlayer[];
}

interface DPNode {
  cost: number;
  score: number;
}

export function findBestSquad(
  players: ScoredPlayer[],
  budgetTenths: number,
  fixturePairings?: Map<number, FixturePairing[]>
): SquadResult {
  const byTeam = new Map<number, ScoredPlayer[]>();
  players.forEach((p) => {
    if (!byTeam.has(p.player.team)) byTeam.set(p.player.team, []);
    byTeam.get(p.player.team)!.push(p);
  });

  const teamIds = Array.from(byTeam.keys()).sort((a, b) => a - b);

  const byPosCostAsc = new Map<number, ScoredPlayer[]>();
  const byPosScoreDesc = new Map<number, ScoredPlayer[]>();

  [PosGK, PosDEF, PosMID, PosFWD].forEach((pos) => {
    const posPlayers = players.filter((p) => p.player.element_type === pos);

    const costAsc = [...posPlayers].sort((a, b) => a.player.now_cost - b.player.now_cost);
    const scoreDesc = [...posPlayers].sort((a, b) => b.score - a.score);

    byPosCostAsc.set(pos, costAsc);
    byPosScoreDesc.set(pos, scoreDesc);
  });

  let bestResult: SquadResult = {
    formation: '',
    starters: [],
    bench: [],
    captain: null,
    viceCaptain: null,
    totalScore: 0,
    xiCost: 0,
    totalCost: 0,
    budget: budgetTenths / 10.0,
  };
  let bestObj = -Infinity;

  for (const fm of VALID_FORMATIONS) {
    const targetStarters = new PosCounts(fm.gk, fm.def, fm.mid, fm.fwd);
    const benchReserve = estimateBenchCost(byPosCostAsc, fm);
    const dpBudget = budgetTenths - benchReserve;
    if (dpBudget <= 0) continue;

    const res = solveDP(teamIds, byTeam, targetStarters, dpBudget, fixturePairings);
    if (!res) continue;

    const { starters, score: xiScore, cost: xiCost } = res;
    if (starters.length !== 11) continue;

    sortByPosAndScore(starters);
    const { captain, viceCaptain } = pickCaptains(starters);

    const xiIds = new Set(starters.map((p) => p.player.id));
    const teamCount = new Map<number, number>();
    const xiPosCounts: Record<number, number> = {
      [PosGK]: 0,
      [PosDEF]: 0,
      [PosMID]: 0,
      [PosFWD]: 0,
    };

    starters.forEach((p) => {
      teamCount.set(p.player.team, (teamCount.get(p.player.team) || 0) + 1);
      xiPosCounts[p.player.element_type] = (xiPosCounts[p.player.element_type] || 0) + 1;
    });

    const bench = fillBench(
      byPosCostAsc,
      byPosScoreDesc,
      xiIds,
      teamCount,
      xiPosCounts,
      budgetTenths - xiCost
    );

    let totalCost = xiCost;
    bench.forEach((p) => {
      totalCost += p.player.now_cost;
    });

    let rawScore = 0;
    starters.forEach((p) => {
      rawScore += p.score;
    });

    if (xiScore > bestObj) {
      bestObj = xiScore;
      bestResult = {
        formation: fm.name,
        starters,
        bench,
        captain,
        viceCaptain,
        totalScore: rawScore,
        xiCost: xiCost / 10.0,
        totalCost: totalCost / 10.0,
        budget: budgetTenths / 10.0,
      };
    }
  }

  return bestResult;
}

function solveDP(
  teamIds: number[],
  byTeam: Map<number, ScoredPlayer[]>,
  target: PosCounts,
  budget: number,
  fixturePairings?: Map<number, FixturePairing[]>
): { starters: ScoredPlayer[]; score: number; cost: number } | null {
  const allOpts: TeamOption[][] = teamIds.map((tid) =>
    generateTeamOptions(byTeam.get(tid) || [], target)
  );

  if (fixturePairings) {
    applyClashDiscounts(teamIds, allOpts, byTeam, fixturePairings);
  }

  const stages: Map<string, DPNode[]>[] = new Array(teamIds.length + 1)
    .fill(null)
    .map(() => new Map<string, DPNode[]>());

  const initCounts = new PosCounts(0, 0, 0, 0);
  stages[0].set(initCounts.key, [{ cost: 0, score: 0 }]);

  for (let ti = 0; ti < teamIds.length; ti++) {
    const opts = allOpts[ti];
    const prev = stages[ti];
    const next = stages[ti + 1];

    prev.forEach((nodes, stateKey) => {
      const parts = stateKey.split(',').map(Number);
      const state = new PosCounts(parts[0], parts[1], parts[2], parts[3]);

      // Skip transition (not picking from this team)
      nodes.forEach((n) => {
        addToFrontier(next, state.key, { cost: n.cost, score: n.score });
      });

      // Pick transitions
      opts.forEach((opt) => {
        const ns = state.add(opt.counts);
        if (!ns.fitsWithin(target)) return;

        nodes.forEach((n) => {
          const nc = n.cost + opt.cost;
          if (nc <= budget) {
            addToFrontier(next, ns.key, { cost: nc, score: n.score + opt.score });
          }
        });
      });
    });
  }

  const finalFrontier = stages[teamIds.length].get(target.key);
  if (!finalFrontier || finalFrontier.length === 0) return null;

  let bestScore = -1;
  let bestCost = 0;
  finalFrontier.forEach((n) => {
    if (n.score > bestScore) {
      bestScore = n.score;
      bestCost = n.cost;
    }
  });

  const starters = reconstructStarters(
    teamIds,
    allOpts,
    stages,
    target,
    bestScore,
    bestCost
  );

  return { starters, score: bestScore, cost: bestCost };
}

function reconstructStarters(
  teamIds: number[],
  allOpts: TeamOption[][],
  stages: Map<string, DPNode[]>[],
  target: PosCounts,
  bestScore: number,
  bestCost: number
): ScoredPlayer[] {
  let starters: ScoredPlayer[] = [];
  let remainScore = bestScore;
  let remainCost = bestCost;
  let remainState = target;

  for (let ti = teamIds.length - 1; ti >= 0; ti--) {
    const prevStage = stages[ti];

    let found = false;
    for (const opt of allOpts[ti]) {
      const prevStateGK = remainState.sGK - opt.counts.sGK;
      const prevStateDEF = remainState.sDEF - opt.counts.sDEF;
      const prevStateMID = remainState.sMID - opt.counts.sMID;
      const prevStateFWD = remainState.sFWD - opt.counts.sFWD;

      if (
        prevStateGK < 0 ||
        prevStateDEF < 0 ||
        prevStateMID < 0 ||
        prevStateFWD < 0
      ) {
        continue;
      }

      const prevState = new PosCounts(
        prevStateGK,
        prevStateDEF,
        prevStateMID,
        prevStateFWD
      );

      const wantCost = remainCost - opt.cost;
      const wantScore = remainScore - opt.score;

      const nodes = prevStage.get(prevState.key);
      if (nodes && matchNode(nodes, wantCost, wantScore)) {
        starters = starters.concat(opt.players);
        remainScore = wantScore;
        remainCost = wantCost;
        remainState = prevState;
        found = true;
        break;
      }
    }

    if (!found) {
      // Fallback: team was skipped
      const nodes = prevStage.get(remainState.key);
      if (nodes) {
        matchNode(nodes, remainCost, remainScore);
      }
    }
  }

  return starters;
}

function matchNode(nodes: DPNode[], wantCost: number, wantScore: number): boolean {
  for (const n of nodes) {
    if (n.cost === wantCost && Math.abs(n.score - wantScore) < 1e-6) {
      return true;
    }
  }
  return false;
}

function addToFrontier(
  dp: Map<string, DPNode[]>,
  stateKey: string,
  node: DPNode
): void {
  const existing = dp.get(stateKey) || [];

  for (const e of existing) {
    if (
      (e.cost < node.cost && e.score >= node.score) ||
      (e.cost <= node.cost && e.score > node.score)
    ) {
      return;
    }
  }

  const kept = existing.filter(
    (e) =>
      !(
        (node.cost < e.cost && node.score >= e.score) ||
        (node.cost <= e.cost && node.score > e.score)
      )
  );

  kept.push(node);
  dp.set(stateKey, kept);
}

function generateTeamOptions(pool: ScoredPlayer[], target: PosCounts): TeamOption[] {
  if (pool.length === 0) return [];

  const posPool = new Map<number, ScoredPlayer[]>();
  pool.forEach((p) => {
    if (!posPool.has(p.player.element_type)) posPool.set(p.player.element_type, []);
    posPool.get(p.player.element_type)!.push(p);
  });

  const limited: ScoredPlayer[] = [];
  [PosGK, PosDEF, PosMID, PosFWD].forEach((pos) => {
    const pp = posPool.get(pos) || [];
    pp.sort((a, b) => b.score - a.score);
    limited.push(...pp.slice(0, 3));
  });

  const opts: TeamOption[] = [];
  const n = limited.length;
  const idx: number[] = [0, 0, 0];

  for (let size = 1; size <= 3 && size <= n; size++) {
    enumerateStarterSubsets(limited, size, 0, idx, 0, target, opts);
  }

  return pruneTeamOptions(opts);
}

function enumerateStarterSubsets(
  pool: ScoredPlayer[],
  size: number,
  start: number,
  idx: number[],
  depth: number,
  target: PosCounts,
  opts: TeamOption[]
): void {
  if (depth === size) {
    const players: ScoredPlayer[] = [];
    let sGK = 0,
      sDEF = 0,
      sMID = 0,
      sFWD = 0;
    let totalCost = 0;
    let totalScore = 0;

    for (let d = 0; d < depth; d++) {
      const p = pool[idx[d]];
      players.push(p);
      totalCost += p.player.now_cost;
      totalScore += p.score;

      switch (p.player.element_type) {
        case PosGK:
          sGK++;
          break;
        case PosDEF:
          sDEF++;
          break;
        case PosMID:
          sMID++;
          break;
        case PosFWD:
          sFWD++;
          break;
      }
    }

    const counts = new PosCounts(sGK, sDEF, sMID, sFWD);
    if (!counts.fitsWithin(target)) return;

    opts.push({
      counts,
      cost: totalCost,
      score: totalScore,
      players,
    });
    return;
  }

  for (let i = start; i < pool.length; i++) {
    idx[depth] = i;
    enumerateStarterSubsets(pool, size, i + 1, idx, depth + 1, target, opts);
  }
}

function pruneTeamOptions(opts: TeamOption[]): TeamOption[] {
  const groups = new Map<string, TeamOption[]>();
  opts.forEach((o) => {
    const key = o.counts.key;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(o);
  });

  const pruned: TeamOption[] = [];
  groups.forEach((group) => {
    group.sort((a, b) => (a.cost !== b.cost ? a.cost - b.cost : b.score - a.score));

    let bestScore = -1;
    for (const o of group) {
      if (o.score > bestScore) {
        pruned.push(o);
        bestScore = o.score;
      }
    }
  });

  return pruned;
}

function applyClashDiscounts(
  teamIds: number[],
  allOpts: TeamOption[][],
  byTeam: Map<number, ScoredPlayer[]>,
  pairings: Map<number, FixturePairing[]>
): void {
  const clashWeight = 0.5;

  const teamHasAtk = new Map<number, boolean>();
  byTeam.forEach((players, teamId) => {
    const hasAtk = players.some(
      (p) => p.player.element_type === PosMID || p.player.element_type === PosFWD
    );
    teamHasAtk.set(teamId, hasAtk);
  });

  for (let i = 0; i < teamIds.length; i++) {
    const teamId = teamIds[i];
    const teamPairings = pairings.get(teamId) || [];

    let maxFDR = 0;
    let hasOppAtk = false;

    for (const fp of teamPairings) {
      if (teamHasAtk.get(fp.opponentId)) {
        hasOppAtk = true;
        if (fp.difficulty > maxFDR) maxFDR = fp.difficulty;
      }
    }

    if (!hasOppAtk) continue;

    const fdrScale = maxFDR / 5.0;
    allOpts[i].forEach((opt) => {
      let penalty = 0;
      opt.players.forEach((p) => {
        if (p.player.element_type === PosGK || p.player.element_type === PosDEF) {
          penalty += clashWeight * fdrScale * p.score;
        }
      });
      opt.score -= penalty;
    });
  }
}

function estimateBenchCost(
  byPosCostAsc: Map<number, ScoredPlayer[]>,
  fm: Formation
): number {
  const benchNeeds: Record<number, number> = {
    [PosGK]: SQUAD_SLOTS[PosGK] - fm.gk,
    [PosDEF]: SQUAD_SLOTS[PosDEF] - fm.def,
    [PosMID]: SQUAD_SLOTS[PosMID] - fm.mid,
    [PosFWD]: SQUAD_SLOTS[PosFWD] - fm.fwd,
  };

  let total = 0;
  [PosGK, PosDEF, PosMID, PosFWD].forEach((pos) => {
    const need = benchNeeds[pos];
    const sorted = byPosCostAsc.get(pos) || [];
    for (let i = 0; i < need && i < sorted.length; i++) {
      total += sorted[i].player.now_cost;
    }
  });

  return total;
}

function fillBench(
  byPosCostAsc: Map<number, ScoredPlayer[]>,
  byPosScoreDesc: Map<number, ScoredPlayer[]>,
  xiIds: Set<number>,
  teamCount: Map<number, number>,
  xiPosCounts: Record<number, number>,
  remainingBudgetTenths: number
): ScoredPlayer[] {
  const remBudget = Math.floor(remainingBudgetTenths);

  const benchNeeds: Record<number, number> = {
    [PosGK]: SQUAD_SLOTS[PosGK] - (xiPosCounts[PosGK] || 0),
    [PosDEF]: SQUAD_SLOTS[PosDEF] - (xiPosCounts[PosDEF] || 0),
    [PosMID]: SQUAD_SLOTS[PosMID] - (xiPosCounts[PosMID] || 0),
    [PosFWD]: SQUAD_SLOTS[PosFWD] - (xiPosCounts[PosFWD] || 0),
  };

  interface PosGroup {
    pos: number;
    need: number;
    candidates: ScoredPlayer[];
  }

  const groups: PosGroup[] = [];
  const positions = [PosGK, PosDEF, PosMID, PosFWD];

  for (const pos of positions) {
    const need = benchNeeds[pos] || 0;
    if (need <= 0) continue;

    const filtered: ScoredPlayer[] = [];
    const seen = new Set<number>();

    const scoreDesc = byPosScoreDesc.get(pos) || [];
    for (const p of scoreDesc) {
      if (xiIds.has(p.player.id) || (teamCount.get(p.player.team) || 0) >= 3) continue;
      if (p.player.now_cost > remBudget) continue;
      filtered.push(p);
      seen.add(p.player.id);
    }

    const costAsc = byPosCostAsc.get(pos) || [];
    for (const p of costAsc) {
      if (seen.has(p.player.id) || xiIds.has(p.player.id) || (teamCount.get(p.player.team) || 0) >= 3) continue;
      if (p.player.now_cost > remBudget) continue;
      filtered.push(p);
      seen.add(p.player.id);
    }

    if (filtered.length > 25) {
      filtered.sort((a, b) => b.score - a.score);
      const topByScore = filtered.slice(0, 20);
      const rest = filtered.slice(20);
      rest.sort((a, b) => a.player.now_cost - b.player.now_cost);
      const candidates = [...topByScore, ...rest.slice(0, 5)];
      candidates.sort((a, b) => b.score - a.score);
      groups.push({ pos, need, candidates });
    } else {
      groups.push({ pos, need, candidates: filtered });
    }
  }

  if (groups.length === 0) return [];

  const minCostSuffix: number[] = new Array(groups.length + 1).fill(0);
  const maxScoreSuffix: number[] = new Array(groups.length + 1).fill(0);

  for (let i = groups.length - 1; i >= 0; i--) {
    const need = groups[i].need;
    const candidates = groups[i].candidates;
    let minCost = 0;
    let maxScore = 0;

    if (candidates.length >= need) {
      const costs = candidates.map((p) => p.player.now_cost).sort((a, b) => a - b);
      const scores = candidates.map((p) => p.score).sort((a, b) => b - a);
      for (let k = 0; k < need; k++) {
        minCost += costs[k];
        maxScore += scores[k];
      }
    }
    minCostSuffix[i] = minCostSuffix[i + 1] + minCost;
    maxScoreSuffix[i] = maxScoreSuffix[i + 1] + maxScore;
  }

  let bestScore = -1;
  let bestCost = Number.MAX_SAFE_INTEGER;
  let bestBench: ScoredPlayer[] = [];

  const tc = new Map<number, number>(teamCount);

  function search(gIdx: number, curCost: number, curScore: number, curBench: ScoredPlayer[]) {
    if (gIdx === groups.length) {
      if (curScore > bestScore + 1e-9 || (Math.abs(curScore - bestScore) <= 1e-9 && curCost < bestCost)) {
        bestScore = curScore;
        bestCost = curCost;
        bestBench = [...curBench];
      }
      return;
    }

    const g = groups[gIdx];
    const cands = g.candidates;

    if (curCost + minCostSuffix[gIdx] > remBudget) return;
    if (bestScore >= 0 && curScore + maxScoreSuffix[gIdx] < bestScore - 1e-9) return;

    if (g.need === 1) {
      for (const p of cands) {
        const cost = curCost + p.player.now_cost;
        if (cost + minCostSuffix[gIdx + 1] > remBudget) continue;
        const currentTc = tc.get(p.player.team) || 0;
        if (currentTc >= 3) continue;

        tc.set(p.player.team, currentTc + 1);
        search(gIdx + 1, cost, curScore + p.score, [...curBench, p]);
        tc.set(p.player.team, currentTc);
      }
    } else if (g.need === 2) {
      for (let i = 0; i < cands.length; i++) {
        const p1 = cands[i];
        const tc1 = tc.get(p1.player.team) || 0;
        if (tc1 >= 3) continue;
        tc.set(p1.player.team, tc1 + 1);

        for (let j = i + 1; j < cands.length; j++) {
          const p2 = cands[j];
          const tc2 = tc.get(p2.player.team) || 0;
          if (tc2 >= 3) continue;
          const cost = curCost + p1.player.now_cost + p2.player.now_cost;
          if (cost + minCostSuffix[gIdx + 1] > remBudget) continue;

          tc.set(p2.player.team, tc2 + 1);
          search(gIdx + 1, cost, curScore + p1.score + p2.score, [...curBench, p1, p2]);
          tc.set(p2.player.team, tc2);
        }

        tc.set(p1.player.team, tc1);
      }
    }
  }

  search(0, 0, 0, []);

  if (bestBench.length === 0) {
    return fallbackFillBench(byPosCostAsc, xiIds, teamCount, benchNeeds, remBudget);
  }

  const gkBench: ScoredPlayer[] = [];
  const outfieldBench: ScoredPlayer[] = [];
  for (const p of bestBench) {
    if (p.player.element_type === PosGK) {
      gkBench.push(p);
    } else {
      outfieldBench.push(p);
    }
  }
  outfieldBench.sort((a, b) => b.score - a.score);

  const finalBench = [...gkBench, ...outfieldBench];
  finalBench.forEach((p) => {
    xiIds.add(p.player.id);
    teamCount.set(p.player.team, (teamCount.get(p.player.team) || 0) + 1);
  });

  return finalBench;
}

function fallbackFillBench(
  byPosCostAsc: Map<number, ScoredPlayer[]>,
  xiIds: Set<number>,
  teamCount: Map<number, number>,
  benchNeeds: Record<number, number>,
  remBudget: number
): ScoredPlayer[] {
  const bench: ScoredPlayer[] = [];
  let rem = remBudget;

  [PosGK, PosDEF, PosMID, PosFWD].forEach((pos) => {
    const pool = byPosCostAsc.get(pos) || [];
    const need = benchNeeds[pos] || 0;
    let cnt = 0;

    for (const p of pool) {
      if (cnt >= need) break;
      if (xiIds.has(p.player.id) || (teamCount.get(p.player.team) || 0) >= 3) continue;
      if (p.player.now_cost > rem) continue;

      bench.push(p);
      xiIds.add(p.player.id);
      teamCount.set(p.player.team, (teamCount.get(p.player.team) || 0) + 1);
      rem -= p.player.now_cost;
      cnt++;
    }
  });

  return bench;
}

export function pickCaptains(starters: ScoredPlayer[]): {
  captain: ScoredPlayer | null;
  viceCaptain: ScoredPlayer | null;
} {
  if (starters.length === 0) {
    return { captain: null, viceCaptain: null };
  }

  const sorted = [...starters].sort((a, b) => b.score - a.score);
  const eligible = sorted.filter(
    (p) => p.player.chance_of_playing_next_round !== 0
  );

  if (eligible.length === 0) {
    return {
      captain: sorted[0] || null,
      viceCaptain: sorted[1] || null,
    };
  }

  return {
    captain: eligible[0] || null,
    viceCaptain: eligible[1] || null,
  };
}

export function sortByPosAndScore(players: ScoredPlayer[]): void {
  players.sort((a, b) => {
    if (a.player.element_type !== b.player.element_type) {
      return a.player.element_type - b.player.element_type;
    }
    return b.score - a.score;
  });
}

export function bestXIFromSquad(squad: ScoredPlayer[]): SquadResult {
  const byPos = new Map<number, ScoredPlayer[]>();
  [PosGK, PosDEF, PosMID, PosFWD].forEach((pos) => byPos.set(pos, []));

  squad.forEach((p) => {
    const list = byPos.get(p.player.element_type);
    if (list) list.push(p);
  });

  byPos.forEach((pool) => {
    pool.sort((a, b) => b.score - a.score);
  });

  let bestStarters: ScoredPlayer[] = [];
  let bestFormation = '';
  let maxScore = -1;

  for (const fm of VALID_FORMATIONS) {
    const needs: Record<number, number> = {
      [PosGK]: fm.gk,
      [PosDEF]: fm.def,
      [PosMID]: fm.mid,
      [PosFWD]: fm.fwd,
    };

    let valid = true;
    for (const pos of [PosGK, PosDEF, PosMID, PosFWD]) {
      if ((byPos.get(pos)?.length || 0) < needs[pos]) {
        valid = false;
        break;
      }
    }
    if (!valid) continue;

    const trial: ScoredPlayer[] = [];
    let score = 0;

    for (const pos of [PosGK, PosDEF, PosMID, PosFWD]) {
      const n = needs[pos];
      const pool = byPos.get(pos) || [];
      for (let j = 0; j < n; j++) {
        trial.push(pool[j]);
        score += pool[j].score;
      }
    }

    if (score > maxScore) {
      maxScore = score;
      bestStarters = trial;
      bestFormation = fm.name;
    }
  }

  sortByPosAndScore(bestStarters);

  const starterIds = new Set(bestStarters.map((p) => p.player.id));
  const benchUnsorted = squad.filter((p) => !starterIds.has(p.player.id));
  const benchGk = benchUnsorted.filter((p) => p.player.element_type === PosGK);
  const benchOutfield = benchUnsorted.filter((p) => p.player.element_type !== PosGK);
  benchOutfield.sort((a, b) => b.score - a.score);

  const bench = [...benchGk, ...benchOutfield];

  const { captain, viceCaptain } = pickCaptains(bestStarters);

  let xiCost = 0;
  bestStarters.forEach((p) => (xiCost += p.player.now_cost));

  let totalCost = xiCost;
  bench.forEach((p) => (totalCost += p.player.now_cost));

  return {
    formation: bestFormation,
    starters: bestStarters,
    bench,
    captain,
    viceCaptain,
    totalScore: maxScore,
    xiCost: xiCost / 10.0,
    totalCost: totalCost / 10.0,
    budget: totalCost / 10.0,
  };
}
