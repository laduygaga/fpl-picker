import type { SquadResult } from '../lib/model/recommender';
import type { TransferSuggestion } from '../lib/model/transfers';

interface OptimizeResponse {
  success: boolean;
  error?: string;
  data?: {
    optimal: SquadResult;
    transfers: TransferSuggestion[];
    nextGw: number;
    scoredPlayersCount: number;
  };
}

let lastOptimizationData: {
  optimal: SquadResult;
  transfers: TransferSuggestion[];
  nextGw: number;
} | null = null;

document.addEventListener('DOMContentLoaded', () => {
  setupTabs();
  autoDetectTeamId();

  document.getElementById('run-btn')?.addEventListener('click', runOptimizer);
  document.getElementById('apply-btn')?.addEventListener('click', applyChangesToFPL);
});

function setupTabs(): void {
  const tabBtns = document.querySelectorAll('.tab-btn');
  tabBtns.forEach((btn) => {
    btn.addEventListener('click', () => {
      tabBtns.forEach((b) => b.classList.remove('active'));
      btn.classList.add('active');

      const targetTab = btn.getAttribute('data-tab');
      document.querySelectorAll('.tab-content').forEach((content) => {
        (content as HTMLElement).style.display = 'none';
      });

      if (targetTab) {
        const activeContent = document.getElementById(targetTab);
        if (activeContent) activeContent.style.display = 'block';
      }
    });
  });
}

function autoDetectTeamId(): void {
  chrome.runtime.sendMessage({ type: 'DETECT_ENTRY' }, (res) => {
    if (res?.success && res.entry) {
      const input = document.getElementById('team-id') as HTMLInputElement;
      if (input && !input.value) {
        input.value = res.entry.toString();
      }
    }
  });
}

function runOptimizer(): void {
  const output = document.getElementById('optimal-output');
  const transfersOutput = document.getElementById('transfers-output');
  const runBtn = document.getElementById('run-btn') as HTMLButtonElement;

  if (output) {
    output.innerHTML = `<div class="loading-spinner">Scoring players & running DP optimizer...</div>`;
  }
  if (transfersOutput) {
    transfersOutput.innerHTML = `<div class="loading-spinner">Planning optimal transfers...</div>`;
  }

  if (runBtn) runBtn.disabled = true;

  const budget = parseFloat((document.getElementById('budget') as HTMLInputElement)?.value || '100.0');
  const formula = (document.getElementById('formula') as HTMLSelectElement)?.value || '1';
  const teamIdInput = (document.getElementById('team-id') as HTMLInputElement)?.value;
  const teamId = teamIdInput ? parseInt(teamIdInput, 10) : undefined;

  chrome.runtime.sendMessage(
    {
      type: 'OPTIMIZE_SQUAD',
      budget,
      formula,
      teamId,
    },
    (res: OptimizeResponse) => {
      if (runBtn) runBtn.disabled = false;

      if (!res || !res.success || !res.data) {
        const errorMsg = res?.error || 'Failed to calculate optimization.';
        if (output) output.innerHTML = `<div style="color: #ff5555; padding: 20px;">Error: ${errorMsg}</div>`;
        return;
      }

      const { optimal, transfers, nextGw } = res.data;
      lastOptimizationData = { optimal, transfers, nextGw };

      // Update GW Badge
      const gwBadge = document.getElementById('gw-badge');
      if (gwBadge) gwBadge.textContent = `GW${nextGw}`;

      const summaryBanner = document.getElementById('summary-banner');
      if (summaryBanner) summaryBanner.style.display = 'flex';

      const applyBtn = document.getElementById('apply-btn') as HTMLButtonElement;
      if (applyBtn && teamId) {
        applyBtn.style.display = 'block';
        applyBtn.disabled = false;
        applyBtn.textContent = '⚡ Apply Changes to FPL Team';
      }

      const xiScoreEl = document.getElementById('xi-score');
      if (xiScoreEl) xiScoreEl.textContent = optimal.totalScore.toFixed(3);

      const formationEl = document.getElementById('formation-label');
      if (formationEl) formationEl.textContent = `${optimal.formation} Formation`;

      const costEl = document.getElementById('cost-label');
      if (costEl) costEl.textContent = `XI: £${optimal.xiCost.toFixed(1)}M | Squad: £${optimal.totalCost.toFixed(1)}M`;

      // Update Captain Cards
      const capContainer = document.getElementById('captain-container');
      if (capContainer) capContainer.style.display = 'flex';

      const capName = document.getElementById('captain-name');
      if (capName) capName.textContent = optimal.captain ? `${optimal.captain.player.web_name} (${optimal.captain.teamName})` : 'N/A';

      const vcName = document.getElementById('vc-name');
      if (vcName) vcName.textContent = optimal.viceCaptain ? `${optimal.viceCaptain.player.web_name} (${optimal.viceCaptain.teamName})` : 'N/A';

      // Render Tables
      renderOptimalTable(optimal);
      renderTransfersTable(transfers);
    }
  );
}

function applyChangesToFPL(): void {
  if (!lastOptimizationData) return;

  const teamIdInput = (document.getElementById('team-id') as HTMLInputElement)?.value;
  if (!teamIdInput) {
    alert('Please enter your FPL Team ID first.');
    return;
  }

  const teamId = parseInt(teamIdInput, 10);
  const applyBtn = document.getElementById('apply-btn') as HTMLButtonElement;
  if (applyBtn) {
    applyBtn.disabled = true;
    applyBtn.textContent = '⏳ Applying changes to FPL...';
  }

  chrome.runtime.sendMessage(
    {
      type: 'APPLY_CHANGES',
      teamId,
      nextGw: lastOptimizationData.nextGw,
      optimal: lastOptimizationData.optimal,
      transfers: lastOptimizationData.transfers,
    },
    (res) => {
      if (applyBtn) {
        if (res && res.success) {
          applyBtn.textContent = '✅ Applied Successfully to FPL!';
          applyBtn.style.background = '#00ff87';
          applyBtn.style.color = '#38003c';
        } else {
          applyBtn.disabled = false;
          applyBtn.textContent = '⚡ Apply Changes to FPL Team';
          alert(`Failed to apply changes: ${res?.error || 'Unknown error'}`);
        }
      }
    }
  );
}

function renderOptimalTable(optimal: SquadResult): void {
  const output = document.getElementById('optimal-output');
  if (!output) return;

  let html = `
    <h3 style="font-size: 13px; color: var(--fpl-green); margin: 12px 0 6px 0;">STARTING XI</h3>
    <table class="player-table">
      <thead>
        <tr>
          <th>POS</th>
          <th>PLAYER</th>
          <th>TEAM</th>
          <th>COST</th>
          <th>SCORE</th>
          <th>OPPONENT</th>
        </tr>
      </thead>
      <tbody>
  `;

  optimal.starters.forEach((sp) => {
    const isC = optimal.captain?.player.id === sp.player.id;
    const isVc = optimal.viceCaptain?.player.id === sp.player.id;
    const badge = isC ? `<span class="badge-c">C</span>` : isVc ? `<span class="badge-vc">VC</span>` : '';

    html += `
      <tr>
        <td><strong>${sp.positionName}</strong></td>
        <td><strong>${sp.player.web_name}</strong> ${badge}</td>
        <td>${sp.teamName}</td>
        <td>£${(sp.player.now_cost / 10).toFixed(1)}M</td>
        <td style="color: var(--fpl-green); font-weight: 700;">${sp.score.toFixed(3)}</td>
        <td style="font-size: 10px; color: var(--fpl-subtext);">${sp.oppDesc}</td>
      </tr>
    `;
  });

  html += `
      </tbody>
    </table>

    <h3 style="font-size: 13px; color: var(--fpl-subtext); margin: 16px 0 6px 0;">BENCH</h3>
    <table class="player-table">
      <thead>
        <tr>
          <th>POS</th>
          <th>PLAYER</th>
          <th>TEAM</th>
          <th>COST</th>
          <th>SCORE</th>
        </tr>
      </thead>
      <tbody>
  `;

  optimal.bench.forEach((sp) => {
    html += `
      <tr>
        <td>${sp.positionName}</td>
        <td>${sp.player.web_name}</td>
        <td>${sp.teamName}</td>
        <td>£${(sp.player.now_cost / 10).toFixed(1)}M</td>
        <td>${sp.score.toFixed(3)}</td>
      </tr>
    `;
  });

  html += `
      </tbody>
    </table>
  `;

  output.innerHTML = html;
}

function renderTransfersTable(transfers: TransferSuggestion[]): void {
  const output = document.getElementById('transfers-output');
  if (!output) return;

  if (!transfers || transfers.length === 0) {
    output.innerHTML = `
      <div style="text-align: center; color: var(--fpl-green); padding: 30px 0;">
        ✨ Your squad is already aligned with the optimal pick! No transfers needed.
      </div>
    `;
    return;
  }

  let html = `<h3 style="font-size: 13px; color: var(--fpl-green); margin-bottom: 10px;">PROPOSED TRANSFERS (${transfers.length})</h3>`;

  transfers.forEach((t) => {
    html += `
      <div class="transfer-item">
        <div>
          <div class="transfer-out">OUT: ${t.out.player.web_name || `#${t.out.player.id}`} (£${(t.sellingPrice / 10).toFixed(1)}M)</div>
          <div class="transfer-in">IN: ${t.in.player.web_name} (${t.in.teamName}, ${t.in.positionName}, £${(t.purchasePrice / 10).toFixed(1)}M)</div>
        </div>
        <div style="text-align: right;">
          <div style="font-weight: 700; color: var(--fpl-green);">+${t.scoreUplift.toFixed(3)}</div>
          <div style="font-size: 10px; color: var(--fpl-subtext);">Score Uplift</div>
        </div>
      </div>
    `;
  });

  output.innerHTML = html;
}
