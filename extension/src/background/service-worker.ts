import { fplClient } from '../lib/api/client';
import { Scorer } from '../lib/model/scorer';
import { findBestSquad, bestXIFromSquad } from '../lib/model/recommender';
import { planTransfers } from '../lib/model/transfers';
import { planLineup } from '../lib/model/lineup';
import type { TransferSuggestion } from '../lib/model/transfers';
import type { SquadResult } from '../lib/model/recommender';
import type { TransferRequest } from '../lib/api/types';

chrome.runtime.onInstalled.addListener(() => {
  console.log('FPL Picker AI extension installed.');
});

interface OptimizeMessage {
  type: 'OPTIMIZE_SQUAD';
  budget: number; // £M e.g. 100.0 or 102.1
  formula: string; // '1' | '2' | '3' | 'balanced' | 'attacker' | 'defender'
  teamId?: number | null;
  fresh?: boolean;
}

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message.type === 'FETCH_BOOTSTRAP') {
    fplClient
      .getBootstrapStatic(message.fresh)
      .then((data) => sendResponse({ success: true, data }))
      .catch((err) => sendResponse({ success: false, error: err.message }));
    return true; // Keep message channel open for async response
  }

  if (message.type === 'FETCH_FIXTURES') {
    fplClient
      .getFixtures(message.fresh)
      .then((data) => sendResponse({ success: true, data }))
      .catch((err) => sendResponse({ success: false, error: err.message }));
    return true;
  }

  if (message.type === 'DETECT_ENTRY') {
    fplClient
      .getMyEntryId()
      .then((entry) => sendResponse({ success: true, entry }))
      .catch((err) => sendResponse({ success: false, error: err.message }));
    return true;
  }

  if (message.type === 'FETCH_MY_TEAM') {
    fplClient
      .getMyTeam(message.teamId)
      .then((data) => sendResponse({ success: true, data }))
      .catch((err) => sendResponse({ success: false, error: err.message }));
    return true;
  }

  if (message.type === 'OPTIMIZE_SQUAD') {
    const { budget, formula, teamId, fresh } = message as OptimizeMessage;

    (async () => {
      try {
        let tid = teamId;
        if (!tid) {
          tid = await fplClient.getMyEntryId();
        }

        const [bootstrap, fixtures, myTeam] = await Promise.all([
          fplClient.getBootstrapStatic(fresh),
          fplClient.getFixtures(fresh),
          tid ? fplClient.getMyTeam(tid).catch(() => null) : Promise.resolve(null),
        ]);

        const scorer = new Scorer(
          bootstrap.teams,
          fixtures,
          bootstrap.events,
          bootstrap.elements,
          formula
        );

        const scoredPlayers = scorer.scoreAll(bootstrap.elements);
        const pairings = scorer.getFixturePairings();

        const budgetTenths = Math.round(budget * 10);
        const optimal = findBestSquad(scoredPlayers, budgetTenths, pairings);

        const idToTeam = new Map<number, number>();
        bootstrap.elements.forEach((p) => idToTeam.set(p.id, p.team));

        const transfers = myTeam ? planTransfers(myTeam, optimal, 4, idToTeam) : [];

        sendResponse({
          success: true,
          data: {
            optimal,
            myTeam,
            transfers,
            nextGw: scorer.getNextEventId(),
            scoredPlayersCount: scoredPlayers.length,
            detectedTeamId: tid,
          },
        });
      } catch (err) {
        sendResponse({ success: false, error: (err as Error).message });
      }
    })();

    return true;
  }

  if (message.type === 'APPLY_CHANGES') {
    const { teamId, nextGw, transfers, formula } = message as {
      type: 'APPLY_CHANGES';
      teamId: number;
      nextGw: number;
      formula?: string;
      optimal: SquadResult;
      transfers: TransferSuggestion[];
    };

    (async () => {
      try {
        let transferNotice = '';

        if (transfers && transfers.length > 0) {
          const req: TransferRequest = {
            confirmed: true,
            entry: teamId,
            event: nextGw,
            transfers: transfers.map((t) => ({
              element_in: t.in.player.id,
              element_out: t.out.player.id,
              purchase_price: t.purchasePrice,
              selling_price: t.sellingPrice,
            })),
          };

          const transferRes = await fplClient.postTransfers(req);
          if (!transferRes.success) {
            sendResponse({ success: false, error: `Transfer failed: ${transferRes.error}` });
            return;
          }
          transferNotice = `Transfers committed (${transfers.length}). `;
        }

        const [bootstrap, fixtures, myTeam] = await Promise.all([
          fplClient.getBootstrapStatic(true),
          fplClient.getFixtures(true),
          fplClient.getMyTeam(teamId),
        ]);

        const scorer = new Scorer(
          bootstrap.teams,
          fixtures,
          bootstrap.events,
          bootstrap.elements,
          formula || '1'
        );

        const allScored = scorer.scoreAll(bootstrap.elements);
        const scoredById = new Map<number, typeof allScored[0]>();
        allScored.forEach((sp) => scoredById.set(sp.player.id, sp));

        const current15Scored: typeof allScored = [];
        myTeam.picks.forEach((pick) => {
          const sp = scoredById.get(pick.element);
          if (sp) {
            current15Scored.push(sp);
          }
        });

        const postTransferResult = bestXIFromSquad(current15Scored);
        const lineup = planLineup(postTransferResult);

        const lineupRes = await fplClient.postLineup(teamId, lineup);
        if (!lineupRes.success) {
          sendResponse({
            success: false,
            error: `${transferNotice}Lineup update failed: ${lineupRes.error}`,
          });
          return;
        }

        sendResponse({
          success: true,
          message: `${transferNotice}Lineup & Captains applied successfully to FPL!`,
        });
      } catch (err) {
        sendResponse({ success: false, error: (err as Error).message });
      }
    })();

    return true;
  }

  return false;
});
