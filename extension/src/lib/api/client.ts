import type {
  BootstrapStaticResponse,
  Fixture,
  MyTeam,
  LineupUpdate,
  TransferRequest,
  TransferErrorResponse,
} from './types';

const FPL_BASE_URL = 'https://fantasy.premierleague.com/api';

interface CacheEntry<T> {
  data: T;
  timestamp: number;
}

const BOOTSTRAP_CACHE_KEY = 'fpl_bootstrap_static_cache';
const FIXTURES_CACHE_KEY = 'fpl_fixtures_cache';
const CACHE_TTL_MS = 60 * 60 * 1000; // 1 hour

export class FPLClient {
  /**
   * Fetches the main bootstrap static data (players, teams, events).
   * Uses chrome.storage.local caching unless `fresh` is true.
   */
  async getBootstrapStatic(fresh = false): Promise<BootstrapStaticResponse> {
    if (!fresh && typeof chrome !== 'undefined' && chrome.storage?.local) {
      const cached = await chrome.storage.local.get(BOOTSTRAP_CACHE_KEY);
      const entry = cached[BOOTSTRAP_CACHE_KEY] as CacheEntry<BootstrapStaticResponse> | undefined;
      if (entry && Date.now() - entry.timestamp < CACHE_TTL_MS) {
        return entry.data;
      }
    }

    const res = await fetch(`${FPL_BASE_URL}/bootstrap-static/`);
    if (!res.ok) {
      throw new Error(`Failed to fetch bootstrap-static: ${res.status} ${res.statusText}`);
    }

    const data = (await res.json()) as BootstrapStaticResponse;

    if (typeof chrome !== 'undefined' && chrome.storage?.local) {
      await chrome.storage.local.set({
        [BOOTSTRAP_CACHE_KEY]: {
          data,
          timestamp: Date.now(),
        },
      });
    }

    return data;
  }

  /**
   * Fetches all fixtures for the season.
   */
  async getFixtures(fresh = false): Promise<Fixture[]> {
    if (!fresh && typeof chrome !== 'undefined' && chrome.storage?.local) {
      const cached = await chrome.storage.local.get(FIXTURES_CACHE_KEY);
      const entry = cached[FIXTURES_CACHE_KEY] as CacheEntry<Fixture[]> | undefined;
      if (entry && Date.now() - entry.timestamp < CACHE_TTL_MS) {
        return entry.data;
      }
    }

    const res = await fetch(`${FPL_BASE_URL}/fixtures/`);
    if (!res.ok) {
      throw new Error(`Failed to fetch fixtures: ${res.status} ${res.statusText}`);
    }

    const data = (await res.json()) as Fixture[];

    if (typeof chrome !== 'undefined' && chrome.storage?.local) {
      await chrome.storage.local.set({
        [FIXTURES_CACHE_KEY]: {
          data,
          timestamp: Date.now(),
        },
      });
    }

    return data;
  }

  /**
   * Attempts to detect the logged in user's team ID from /api/me/.
   */
  async getMyEntryId(): Promise<number | null> {
    try {
      const headers = await this.getAuthHeaders();
      const res = await fetch(`${FPL_BASE_URL}/me/`, {
        headers,
        credentials: 'include',
      });
      if (!res.ok) {
        return null;
      }
      const data = await res.json();
      if (data?.player?.entry) {
        return Number(data.player.entry);
      }
      return null;
    } catch {
      return null;
    }
  }

  /**
   * Fetches user's current squad / picks / bank / transfers info for a given team ID.
   */
  async getMyTeam(teamId: number): Promise<MyTeam> {
    const headers = await this.getAuthHeaders();
    const res = await fetch(`${FPL_BASE_URL}/my-team/${teamId}/`, {
      headers,
      credentials: 'include',
    });
    if (!res.ok) {
      throw new Error(`Failed to fetch my-team for ID ${teamId}: ${res.status} ${res.statusText}`);
    }

    const data = (await res.json()) as MyTeam;
    data.entry_id = teamId;
    return data;
  }

  /**
   * Posts transfers to /api/transfers/.
   * Setting req.confirmed = false validates without saving; true commits transfers.
   */
  async postTransfers(req: TransferRequest): Promise<{ success: boolean; data?: unknown; error?: string }> {
    try {
      const headers = await this.getAuthHeaders();

      const res = await fetch(`${FPL_BASE_URL}/transfers/`, {
        method: 'POST',
        headers,
        credentials: 'include',
        body: JSON.stringify(req),
      });

      const data = await res.json();
      if (!res.ok) {
        const errResp = data as TransferErrorResponse;
        const msg = errResp.message || errResp.non_form_errors?.join(', ') || 'Transfer failed';
        return { success: false, error: msg, data };
      }

      return { success: true, data };
    } catch (e) {
      return { success: false, error: (e as Error).message };
    }
  }

  /**
   * Posts lineup updates (captain, vice captain, starting order) to /api/my-team/{teamId}/.
   */
  async postLineup(teamId: number, lineup: LineupUpdate): Promise<{ success: boolean; error?: string }> {
    try {
      const headers = await this.getAuthHeaders();

      const res = await fetch(`${FPL_BASE_URL}/my-team/${teamId}/`, {
        method: 'POST',
        headers,
        credentials: 'include',
        body: JSON.stringify(lineup),
      });

      if (!res.ok) {
        let errText = '';
        try {
          const data = await res.json();
          errText = data?.message || data?.non_field_errors?.join(', ') || JSON.stringify(data);
        } catch {
          errText = await res.text().catch(() => '');
        }
        if (res.status === 403) {
          errText = '403 Forbidden - Please log into fantasy.premierleague.com in your browser';
        }
        return { success: false, error: errText || `HTTP ${res.status}` };
      }

      return { success: true };
    } catch (e) {
      return { success: false, error: (e as Error).message };
    }
  }

  private async getAuthHeaders(): Promise<Record<string, string>> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (typeof chrome !== 'undefined' && chrome.cookies) {
      try {
        const cookies = await chrome.cookies.getAll({ domain: 'premierleague.com' });
        if (cookies && cookies.length > 0) {
          headers['Cookie'] = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
          const csrf = cookies.find((c) => c.name === 'csrftoken');
          if (csrf) {
            headers['X-CSRFToken'] = csrf.value;
          }
        }
      } catch (e) {
        console.warn('Failed to fetch extension cookies:', e);
      }
    } else if (typeof document !== 'undefined') {
      const match = document.cookie.match(/csrftoken=([^;]+)/);
      if (match) {
        headers['X-CSRFToken'] = match[1];
      }
    }

    return headers;
  }
}

export const fplClient = new FPLClient();
