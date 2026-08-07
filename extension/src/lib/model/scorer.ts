import {
  PosGK,
  PosDEF,
  PosMID,
  PosFWD,
  POSITION_NAMES,
  type Player,
  type Team,
  type Fixture,
  type Event,
} from '../api/types';

export interface Formula {
  name: string;
  fdr: number;
  pts: number;
  form: number;
  ep: number;
  ppg: number;
  xgi: number;
  ict: number;
}

export const FORMULAS: Record<string, Formula> = {
  '1': {
    name: 'Balanced',
    fdr: 0.30,
    pts: 0.20,
    form: 0.15,
    ep: 0.05,
    ppg: 0.05,
    xgi: 0.03,
    ict: 0.02,
  },
  '2': {
    name: 'Attacker',
    fdr: 0.15,
    pts: 0.10,
    form: 0.25,
    ep: 0.10,
    ppg: 0.10,
    xgi: 0.15,
    ict: 0.15,
  },
  '3': {
    name: 'Defender',
    fdr: 0.35,
    pts: 0.25,
    form: 0.10,
    ep: 0.05,
    ppg: 0.10,
    xgi: 0.05,
    ict: 0.10,
  },
};

export const FORMULA_ALIASES: Record<string, string> = {
  balanced: '1',
  attacker: '2',
  defender: '3',
};

export function getFormula(idOrName: string): Formula {
  const key = idOrName.trim().toLowerCase();
  const mapped = FORMULA_ALIASES[key] || key;
  return FORMULAS[mapped] || FORMULAS['1'];
}

export interface ScoredPlayer {
  player: Player;
  score: number;
  teamName: string;
  positionName: string;

  epNextVal: number;
  formVal: number;
  ppgVal: number;
  xgiP90: number;
  ictP90: number;

  oppScore: number;
  oppDesc: string;
  hasFixture: boolean;
  isHome: boolean;
  isDgw: boolean;
  fdrVal: number;

  upcomingFixtures: string;
  valueRating: number;
  ownershipPct: number;
}

export interface FixturePairing {
  opponentId: number;
  difficulty: number;
}

interface GWContext {
  oppId: number;
  isHome: boolean;
  oppDefWeakness: number;
  oppAttWeakness: number;
  easyFDR: number;
}

interface RawMetrics {
  ppg: number;
  ep: number;
  form: number;
  xgi90: number;
  ict90: number;
  oppAtk: number;
  oppDef: number;
  totalPts: number;
  easyFDR: number;
}

class Normalizer {
  private min: number;
  private max: number;

  constructor(vals: number[]) {
    if (vals.length === 0) {
      this.min = 0;
      this.max = 1;
      return;
    }
    let lo = vals[0];
    let hi = vals[0];
    for (let i = 1; i < vals.length; i++) {
      if (vals[i] < lo) lo = vals[i];
      if (vals[i] > hi) hi = vals[i];
    }
    this.min = lo;
    this.max = hi;
  }

  normalize(v: number): number {
    const range = this.max - this.min;
    if (range === 0) return 0;
    return Math.max(0, Math.min(1, (v - this.min) / range));
  }
}

export class Scorer {
  private teams: Map<number, Team>;
  private fixtures: Fixture[];
  private nextEventId: number;
  private teamAttackP90: Map<number, number>;
  private teamDefenceP90: Map<number, number>;
  private gwCtx: Map<number, GWContext[]>;
  private formula: Formula;
  private dgwTeams: Set<number>;

  constructor(
    teams: Team[],
    fixtures: Fixture[],
    events: Event[],
    players: Player[],
    formulaId: string
  ) {
    this.teams = new Map();
    teams.forEach((t) => this.teams.set(t.id, t));

    let nextGW = 1;
    for (const e of events) {
      if (e.is_next) {
        nextGW = e.id;
        break;
      }
    }

    this.fixtures = fixtures;
    this.nextEventId = nextGW;
    this.formula = getFormula(formulaId);
    this.dgwTeams = new Set();
    this.teamAttackP90 = new Map();
    this.teamDefenceP90 = new Map();
    this.gwCtx = new Map();

    this.computeTeamStats(players);
    this.buildGWContext();
  }

  public getNextEventId(): number {
    return this.nextEventId;
  }

  public getFixturePairings(): Map<number, FixturePairing[]> {
    const pairings = new Map<number, FixturePairing[]>();
    for (const f of this.fixtures) {
      if (f.event !== this.nextEventId) continue;

      if (!pairings.has(f.team_h)) pairings.set(f.team_h, []);
      pairings.get(f.team_h)!.push({
        opponentId: f.team_a,
        difficulty: f.team_h_difficulty,
      });

      if (!pairings.has(f.team_a)) pairings.set(f.team_a, []);
      pairings.get(f.team_a)!.push({
        opponentId: f.team_h,
        difficulty: f.team_a_difficulty,
      });
    }
    return pairings;
  }

  private computeTeamStats(players: Player[]): void {
    const teamXG = new Map<number, number>();
    const teamXGA = new Map<number, number>();
    const teamGKMinutes = new Map<number, number>();

    for (const p of players) {
      if (p.minutes === 0) continue;
      const xG = parseFloatNum(p.expected_goals);
      teamXG.set(p.team, (teamXG.get(p.team) || 0) + xG);

      if (p.element_type === PosGK && p.minutes > 450) {
        const xGC = parseFloatNum(p.expected_goals_conceded);
        teamXGA.set(p.team, (teamXGA.get(p.team) || 0) + xGC);
      }
      if (p.element_type === PosGK) {
        teamGKMinutes.set(p.team, (teamGKMinutes.get(p.team) || 0) + p.minutes);
      }
    }

    this.teams.forEach((_, id) => {
      let mins = teamGKMinutes.get(id) || 0;
      if (mins < 90) mins = 90;
      const gp = mins / 90.0;
      this.teamAttackP90.set(id, (teamXG.get(id) || 0) / gp);
      this.teamDefenceP90.set(id, (teamXGA.get(id) || 0) / gp);
    });
  }

  private buildGWContext(): void {
    for (const f of this.fixtures) {
      if (f.event !== this.nextEventId) continue;

      const oppAttH = 1.0 / Math.max(0.5, this.teamAttackP90.get(f.team_a) || 1.0);
      const oppDefH = this.teamDefenceP90.get(f.team_a) || 1.0;

      const oppAttA = 1.0 / Math.max(0.5, this.teamAttackP90.get(f.team_h) || 1.0);
      const oppDefA = this.teamDefenceP90.get(f.team_h) || 1.0;

      if (!this.gwCtx.has(f.team_h)) this.gwCtx.set(f.team_h, []);
      this.gwCtx.get(f.team_h)!.push({
        oppId: f.team_a,
        isHome: true,
        oppDefWeakness: oppDefH,
        oppAttWeakness: oppAttH,
        easyFDR: 6 - f.team_h_difficulty,
      });

      if (!this.gwCtx.has(f.team_a)) this.gwCtx.set(f.team_a, []);
      this.gwCtx.get(f.team_a)!.push({
        oppId: f.team_h,
        isHome: false,
        oppDefWeakness: oppDefA,
        oppAttWeakness: oppAttA,
        easyFDR: 6 - f.team_a_difficulty,
      });
    }

    this.gwCtx.forEach((ctxs, teamId) => {
      if (ctxs.length > 1) {
        this.dgwTeams.add(teamId);
      }
    });
  }

  public scoreAll(players: Player[]): ScoredPlayer[] {
    const scored: ScoredPlayer[] = [];
    const raws: RawMetrics[] = [];

    for (const p of players) {
      if (!isEligible(p)) continue;

      const gp = Math.max(1.0, p.minutes / 90.0);
      const ctxList = this.gwCtx.get(p.team) || [];
      const hasFix = ctxList.length > 0;

      let oppAtk = 0;
      let oppDef = 0;
      let easyFDR = 0;
      let oppDesc = 'BLANK';
      let isHome = false;

      if (hasFix) {
        for (const ctx of ctxList) {
          oppAtk += ctx.oppAttWeakness;
          oppDef += ctx.oppDefWeakness;
          easyFDR += ctx.easyFDR;
        }
        const n = ctxList.length;
        oppAtk /= n;
        oppDef /= n;
        easyFDR /= n;
        isHome = ctxList[0].isHome;
        oppDesc = this.describeOpponents(ctxList);
      }

      const raw: RawMetrics = {
        ppg: parseFloatNum(p.points_per_game),
        ep: parseFloatNum(p.ep_next),
        form: parseFloatNum(p.form),
        xgi90: parseFloatNum(p.expected_goal_involvements) / gp,
        ict90: parseFloatNum(p.ict_index) / gp,
        oppAtk,
        oppDef,
        totalPts: p.total_points,
        easyFDR,
      };

      raws.push(raw);

      const teamName = this.teams.get(p.team)?.short_name || '???';
      const positionName = POSITION_NAMES[p.element_type] || '???';

      scored.push({
        player: p,
        score: 0,
        teamName,
        positionName,
        epNextVal: raw.ep,
        formVal: raw.form,
        ppgVal: raw.ppg,
        xgiP90: raw.xgi90,
        ictP90: raw.ict90,
        oppScore: 0,
        oppDesc,
        hasFixture: hasFix,
        isHome,
        isDgw: this.dgwTeams.has(p.team),
        fdrVal: easyFDR,
        upcomingFixtures: this.describeFixtures(p.team),
        valueRating: 0,
        ownershipPct: parseFloatNum(p.selected_by_percent),
      });
    }

    if (raws.length === 0) return scored;

    const nPPG = new Normalizer(raws.map((r) => r.ppg));
    const nEP = new Normalizer(raws.map((r) => r.ep));
    const nForm = new Normalizer(raws.map((r) => r.form));
    const nXGI = new Normalizer(raws.map((r) => r.xgi90));
    const nICT = new Normalizer(raws.map((r) => r.ict90));
    const nOA = new Normalizer(raws.map((r) => r.oppAtk));
    const nOD = new Normalizer(raws.map((r) => r.oppDef));
    const nTP = new Normalizer(raws.map((r) => r.totalPts));
    const nFDR = new Normalizer(raws.map((r) => r.easyFDR));

    for (let i = 0; i < scored.length; i++) {
      if (!scored[i].hasFixture) {
        scored[i].score = 0;
        continue;
      }

      const r = raws[i];
      const norm = {
        ppg: nPPG.normalize(r.ppg),
        ep: nEP.normalize(r.ep),
        form: nForm.normalize(r.form),
        xgi90: nXGI.normalize(r.xgi90),
        ict90: nICT.normalize(r.ict90),
        oppAtk: nOA.normalize(r.oppAtk),
        oppDef: nOD.normalize(r.oppDef),
        totalPts: nTP.normalize(r.totalPts),
        easyFDR: nFDR.normalize(r.easyFDR),
      };

      let baseScore =
        this.formula.fdr * norm.easyFDR +
        this.formula.pts * norm.totalPts +
        this.formula.form * norm.form +
        this.formula.ep * norm.ep +
        this.formula.ppg * norm.ppg +
        this.formula.xgi * norm.xgi90 +
        this.formula.ict * norm.ict90;

      const gp = Math.max(1.0, scored[i].player.minutes / 90.0);
      const { bonus, oppScore } = scorePositionBonus(scored[i].player, norm, gp);
      baseScore += bonus;
      scored[i].oppScore = oppScore;

      if (scored[i].isHome) {
        baseScore += 0.02; // home advantage bonus
      }

      if (scored[i].isDgw) {
        baseScore += 0.20; // double gameweek bonus
      }

      scored[i].score = baseScore;

      const costM = scored[i].player.now_cost / 10.0;
      if (costM > 0) {
        scored[i].valueRating = baseScore / costM;
      }
    }

    return scored;
  }

  private describeOpponents(ctxList: GWContext[]): string {
    return ctxList.map((ctx) => this.describeOpponent(ctx)).join(' + ');
  }

  private describeOpponent(ctx: GWContext): string {
    const oppTeam = this.teams.get(ctx.oppId);
    const shortName = oppTeam ? oppTeam.short_name : '???';
    const atkRating = this.teamAttackP90.get(ctx.oppId) || 0;
    const defRating = this.teamDefenceP90.get(ctx.oppId) || 0;

    let atkLabel = 'Weak Atk';
    if (atkRating > 1.8) atkLabel = 'Strong Atk';
    else if (atkRating > 1.3) atkLabel = 'Avg Atk';

    let defLabel = 'Solid Def';
    if (defRating > 1.5) defLabel = 'Leaky Def';
    else if (defRating > 1.0) defLabel = 'Avg Def';

    const ha = ctx.isHome ? 'H' : 'A';
    return `${shortName}(${ha}) [${atkLabel}, ${defLabel}]`;
  }

  private describeFixtures(teamId: number): string {
    const upcoming: string[] = [];
    for (const f of this.fixtures) {
      if (f.event === null || f.finished) continue;
      const gw = f.event;
      if (gw < this.nextEventId || gw >= this.nextEventId + 3) continue;

      if (f.team_h === teamId) {
        const opp = this.teams.get(f.team_a)?.short_name || '???';
        upcoming.push(`${opp}(H)`);
      } else if (f.team_a === teamId) {
        const opp = this.teams.get(f.team_h)?.short_name || '???';
        upcoming.push(`${opp}(A)`);
      }
    }

    if (upcoming.length === 0) return 'BLANK';
    return upcoming.join(', ');
  }
}

function scorePositionBonus(
  p: Player,
  norm: { oppAtk: number; oppDef: number },
  gp: number
): { bonus: number; oppScore: number } {
  const wOpponentQuality = 0.20;
  const wCleanSheetBonus = 0.04;

  switch (p.element_type) {
    case PosGK: {
      const opp = norm.oppAtk;
      let bonus = wOpponentQuality * opp;
      const xgc = parseFloatNum(p.expected_goals_conceded) / gp;
      if (xgc < 1.0) {
        bonus += wCleanSheetBonus * (1 - xgc);
      }
      return { bonus, oppScore: opp };
    }

    case PosDEF: {
      const opp = 0.70 * norm.oppAtk + 0.30 * norm.oppDef;
      let bonus = wOpponentQuality * opp;
      const xgc = parseFloatNum(p.expected_goals_conceded) / gp;
      if (xgc < 1.0) {
        bonus += wCleanSheetBonus * (1 - xgc);
      }
      bonus += 0.03 * (parseFloatNum(p.expected_assists) / gp);
      return { bonus, oppScore: opp };
    }

    case PosMID: {
      const opp = 0.20 * norm.oppAtk + 0.80 * norm.oppDef;
      let bonus = wOpponentQuality * opp;
      bonus += 0.04 * (parseFloatNum(p.expected_goals) / gp + parseFloatNum(p.expected_assists) / gp);
      return { bonus, oppScore: opp };
    }

    case PosFWD: {
      const opp = 0.10 * norm.oppAtk + 0.90 * norm.oppDef;
      let bonus = wOpponentQuality * opp;
      bonus += 0.07 * (parseFloatNum(p.expected_goals) / gp);
      return { bonus, oppScore: opp };
    }

    default:
      return { bonus: 0, oppScore: 0 };
  }
}

export function isEligible(p: Player): boolean {
  if (p.status === 'i' || p.status === 's' || p.status === 'u') return false;
  if (p.minutes < 90) return false;
  if (p.status === 'd' && p.chance_of_playing_next_round !== null && p.chance_of_playing_next_round < 50) {
    return false;
  }
  return true;
}

export function parseFloatNum(s: string | null | undefined): number {
  if (!s) return 0;
  const v = parseFloat(s);
  return isNaN(v) ? 0 : v;
}

export function findPlayersByName(scored: ScoredPlayer[], names: string[]): ScoredPlayer[] {
  const byName = new Map<string, ScoredPlayer>();
  scored.forEach((s) => byName.set(s.player.web_name.toLowerCase(), s));

  const found: ScoredPlayer[] = [];
  for (const nm of names) {
    const clean = nm.trim().toLowerCase();
    if (!clean) continue;

    let match = byName.get(clean);
    if (!match) {
      for (const [key, sp] of byName.entries()) {
        if (key.includes(clean) || clean.includes(key)) {
          match = sp;
          break;
        }
      }
    }
    if (match) {
      found.push(match);
    }
  }
  return found;
}
