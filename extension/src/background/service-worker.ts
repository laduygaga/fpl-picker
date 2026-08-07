import { fplClient } from '../lib/api/client';
import { Scorer } from '../lib/model/scorer';
import { findBestSquad } from '../lib/model/recommender';
import { planTransfers } from '../lib/model/transfers';

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

    Promise.all([
      fplClient.getBootstrapStatic(fresh),
      fplClient.getFixtures(fresh),
      teamId ? fplClient.getMyTeam(teamId).catch(() => null) : Promise.resolve(null),
    ])
      .then(([bootstrap, fixtures, myTeam]) => {
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

        // Build player ID to team ID map
        const idToTeam = new Map<number, number>();
        bootstrap.elements.forEach((p) => idToTeam.set(p.id, p.team));

        // Plan transfers if user team is available
        const transfers = myTeam
          ? planTransfers(myTeam, optimal, 4, idToTeam)
          : [];

        sendResponse({
          success: true,
          data: {
            optimal,
            myTeam,
            transfers,
            nextGw: scorer.getNextEventId(),
            scoredPlayersCount: scoredPlayers.length,
          },
        });
      })
      .catch((err) => {
        sendResponse({ success: false, error: err.message });
      });

    return true;
  }

  return false;
});
