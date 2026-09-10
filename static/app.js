// LandWalker Game Client
let currentSessionId = null;
let currentGameState = null;
let isMoving = false;
let isRefreshing = false;
let gameRequestId = 0;

// DOM Elements
const gameGrid = document.getElementById('game-grid');
const hpDisplay = document.getElementById('hp-display');
const movesCount = document.getElementById('moves-count');
const elevationDisplay = document.getElementById('elevation-display');
const statusDisplay = document.getElementById('status-display');
const messageLog = document.getElementById('message-log');
const gridDimensionsBadge = document.getElementById('grid-dimensions-badge');
const floorBadge = document.getElementById('floor-badge');
const seedBadge = document.getElementById('seed-badge');
const aggressionBadge = document.getElementById('aggression-badge');
const engineBadge = document.getElementById('engine-badge');
const engineText = document.getElementById('engine-text');

// Modals & Buttons
const btnNewGame = document.getElementById('btn-new-game');
const btnCustomMap = document.getElementById('btn-custom-map');
const modalBackdrop = document.getElementById('modal-backdrop');
const btnCloseModal = document.getElementById('btn-close-modal');
const btnCancelModal = document.getElementById('btn-cancel-modal');
const customMapForm = document.getElementById('custom-map-form');
const rowsInput = document.getElementById('input-rows');
const columnsInput = document.getElementById('input-cols');
const floorsInput = document.getElementById('input-floors');
const difficultyInput = document.getElementById('input-difficulty');
const aggressionInput = document.getElementById('input-aggression');
const seedInput = document.getElementById('input-seed');
const aggressionValue = document.getElementById('aggression-value');
const aggressionDescription = document.getElementById('aggression-description');
const mapSizePreview = document.getElementById('map-size-preview');
const settingsSummary = document.getElementById('settings-summary');
const terrainMode = document.getElementById('terrain-mode');
const btnRandomSeed = document.getElementById('btn-random-seed');
const btnResetTerrain = document.getElementById('btn-reset-terrain');
const settingsError = document.getElementById('settings-error');
const btnStartExpedition = document.getElementById('btn-start-expedition');
const presetButtons = [...document.querySelectorAll('.preset-button')];
const counterButtons = [...document.querySelectorAll('.counter-button')];
const countInputs = {
    high: document.getElementById('count-high'),
    ramp: document.getElementById('count-ramp'),
    ice: document.getElementById('count-ice'),
    hole: document.getElementById('count-hole'),
    conveyor: document.getElementById('count-conveyor'),
    snow: document.getElementById('count-snow'),
    portal: document.getElementById('count-portal'),
    water: document.getElementById('count-water'),
    wall: document.getElementById('count-wall'),
};

// D-Pad
const btnUp = document.getElementById('btn-up');
const btnDown = document.getElementById('btn-down');
const btnLeft = document.getElementById('btn-left');
const btnRight = document.getElementById('btn-right');

// Initialization
document.addEventListener('DOMContentLoaded', async () => {
    setupEventListeners();
    updateSettingsPreview();
    await checkHealth();
    await startNewGame();
    window.setInterval(refreshState, 400);
});

async function checkHealth() {
    try {
        const res = await fetch('/api/health');
        if (res.ok) {
            const data = await res.json();
            engineText.textContent = `${data.engine} Engine`;
            engineBadge.classList.remove('offline');
        }
    } catch (e) {
        engineText.textContent = 'Engine Offline';
        engineBadge.classList.add('offline');
    }
}

async function startNewGame(rows = 12, cols = 16, difficulty = 'normal', seed = null, elementCounts = null, enemyAggression = 70, totalLevels = 3) {
    const requestId = ++gameRequestId;
    try {
        messageLog.textContent = 'Generating procedural world...';
        const body = {
            rows: parseInt(rows),
            columns: parseInt(cols),
            difficulty: difficulty,
            enemyAggression: parseInt(enemyAggression),
            totalLevels: parseInt(totalLevels),
            seed: seed ? parseInt(seed) : undefined
        };
        // Only attach elementCounts if at least one value is specified
        if (elementCounts && Object.keys(elementCounts).length > 0) {
            body.elementCounts = elementCounts;
        }
        const res = await fetch('/api/game/new', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });

        if (!res.ok) throw new Error(await responseError(res));
        const newState = await res.json();
        if (requestId !== gameRequestId) return false;
        currentGameState = newState;
        currentSessionId = currentGameState.sessionId;

        renderGame(currentGameState);
        checkHealth();
        return true;
    } catch (err) {
        messageLog.textContent = 'Error: ' + err.message;
        return false;
    }
}

async function refreshState() {
    if (!currentSessionId || isMoving || isRefreshing || currentGameState?.gameOver) return;
    isRefreshing = true;
    const sessionId = currentSessionId;
    try {
        const res = await fetch(`/api/game/state?sessionId=${encodeURIComponent(sessionId)}`);
        if (!res.ok) return;
        const nextState = await res.json();
        if (sessionId === currentSessionId && nextState.revision > (currentGameState?.revision || 0)) {
            currentGameState = nextState;
            renderGame(nextState);
        }
    } catch {
        // A later poll will retry if the local server is temporarily busy.
    } finally {
        isRefreshing = false;
    }
}

async function handleMove(direction) {
    if (!currentSessionId || isMoving || currentGameState?.gameOver) return;
    isMoving = true;
    const sessionId = currentSessionId;

    try {
        const res = await fetch(`/api/game/move?sessionId=${encodeURIComponent(sessionId)}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ direction })
        });

        if (!res.ok) throw new Error(await responseError(res));
        const nextState = await res.json();
        if (sessionId === currentSessionId) {
            currentGameState = nextState;
            renderGame(nextState);
        }
    } catch (err) {
        messageLog.textContent = 'Error: ' + err.message;
    } finally {
        isMoving = false;
    }
}

function renderGame(state) {
    if (!state) return;

    // 1. Update HUD
    const hearts = '❤️'.repeat(Math.max(0, state.player.hp)) + '🖤'.repeat(Math.max(0, state.player.maxHp - state.player.hp));
    hpDisplay.textContent = hearts || '💀 DEAD';
    movesCount.textContent = state.movesCount;
    elevationDisplay.textContent = state.player.elevation === 1 ? 'Lvl 1 (High Plateau)' : 'Lvl 0 (Ground)';
    messageLog.textContent = state.message;

    // Status Badge
    statusDisplay.className = 'status-badge';
    if (state.gameWon) {
        statusDisplay.textContent = 'VICTORY!';
        statusDisplay.classList.add('status-won');
    } else if (state.gameOver) {
        statusDisplay.textContent = 'GAME OVER';
        statusDisplay.classList.add('status-dead');
    } else {
        statusDisplay.textContent = 'IN PROGRESS';
        statusDisplay.classList.add('status-playing');
    }

    gridDimensionsBadge.textContent = `${state.rows} × ${state.columns} Grid`;
    floorBadge.textContent = `Floor ${state.level} / ${state.totalLevels}`;
    seedBadge.textContent = `Seed: #${state.seed.toString().slice(-6)}`;
    aggressionBadge.textContent = `Bear pursuit: ${state.enemyAggression}%`;

    // 2. Render Board
    gameGrid.innerHTML = '';
    gameGrid.style.gridTemplateColumns = `repeat(${state.columns}, var(--tile-size))`;

    // Dynamic tile size calculation for big boards
    if (state.columns > 20 || state.rows > 16) {
        document.documentElement.style.setProperty('--tile-size', '28px');
    } else {
        document.documentElement.style.setProperty('--tile-size', '38px');
    }

    const playerR = state.player.row;
    const playerC = state.player.column;

    // Entity Map lookup
    const entityMap = new Map();
    state.entities.forEach(ent => {
        if (ent.alive) {
            entityMap.set(`${ent.row},${ent.column}`, ent);
        }
    });

    for (let r = 0; r < state.rows; r++) {
        for (let c = 0; c < state.columns; c++) {
            const tile = state.grid[r][c];
            const tileEl = document.createElement('div');
            tileEl.className = 'tile';

            // Base terrain type class
            const typeName = (tile.typeName || 'ground').toLowerCase();
            tileEl.classList.add(`tile-${typeName}`);

            if (tile.elevation === 1) {
                tileEl.classList.add('tile-elevated');
            }

            if (typeName === 'hole' && tile.holeVisits > 0) {
                tileEl.classList.add('cracked');
            }

            // Entity Overlay
            const isPlayerHere = (r === playerR && c === playerC);
            const entityHere = entityMap.get(`${r},${c}`);

            if (isPlayerHere) {
                tileEl.classList.add('tile-has-player');
                tileEl.textContent = '🧙';
                tileEl.title = `Player (Elevation ${state.player.elevation})`;
            } else if (entityHere) {
                if (entityHere.type === 'bear') {
                    tileEl.classList.add('tile-has-bear');
                    tileEl.textContent = '🐻';
                    tileEl.title = `Bear (${entityHere.mode || 'patrolling'})`;
                } else if (entityHere.type === 'gazelle') {
                    tileEl.classList.add('tile-has-gazelle');
                    tileEl.textContent = '🦌';
                    tileEl.title = `Gazelle (${entityHere.mode || 'wandering'})`;
                }
            } else {
                tileEl.textContent = tile.displayElement || '_';
                const rampDirection = tile.rampDirection ? ` · faces ${tile.rampDirection}` : '';
                tileEl.title = `${tile.typeName} (${r}, ${c})${rampDirection}`;
            }

            gameGrid.appendChild(tileEl);
        }
    }
}

function setupEventListeners() {
    window.addEventListener('keydown', (e) => {
        if (!isSettingsOpen() && e.target.closest?.('input, select, button, summary')) return;
        if (isSettingsOpen()) {
            if (e.key === 'Escape') closeSettings();
            return;
        }
        if (['ArrowUp', 'KeyW'].includes(e.code)) {
            e.preventDefault();
            handleMove('north');
        } else if (['ArrowDown', 'KeyS'].includes(e.code)) {
            e.preventDefault();
            handleMove('south');
        } else if (['ArrowLeft', 'KeyA'].includes(e.code)) {
            e.preventDefault();
            handleMove('west');
        } else if (['ArrowRight', 'KeyD'].includes(e.code)) {
            e.preventDefault();
            handleMove('east');
        }
    });

    btnUp.addEventListener('click', () => handleMove('north'));
    btnDown.addEventListener('click', () => handleMove('south'));
    btnLeft.addEventListener('click', () => handleMove('west'));
    btnRight.addEventListener('click', () => handleMove('east'));

    btnNewGame.addEventListener('click', () => startNewGame());
    btnCustomMap.addEventListener('click', openSettings);
    btnCloseModal.addEventListener('click', closeSettings);
    btnCancelModal.addEventListener('click', closeSettings);
    modalBackdrop.addEventListener('click', (event) => {
        if (event.target === modalBackdrop) closeSettings();
    });

    presetButtons.forEach((button) => {
        button.addEventListener('click', () => {
            rowsInput.value = button.dataset.rows;
            columnsInput.value = button.dataset.columns;
            updateSettingsPreview();
        });
    });

    counterButtons.forEach((button) => {
        button.addEventListener('click', () => {
            const input = document.getElementById(button.dataset.counter);
            const minimum = Number(input.min) || 0;
            const maximum = Number(input.max) || Number.MAX_SAFE_INTEGER;
            const current = input.value === '' ? 0 : Number(input.value);
            input.value = Math.min(maximum, Math.max(minimum, current + Number(button.dataset.step)));
            updateSettingsPreview();
        });
    });

    customMapForm.addEventListener('input', updateSettingsPreview);
    btnRandomSeed.addEventListener('click', () => {
        seedInput.value = crypto.getRandomValues(new Uint32Array(1))[0];
        seedInput.focus();
    });
    btnResetTerrain.addEventListener('click', () => {
        Object.values(countInputs).forEach((input) => { input.value = ''; });
        updateSettingsPreview();
    });

    customMapForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const elementCounts = {};
        for (const [key, el] of Object.entries(countInputs)) {
            if (el && el.value !== '') {
                const v = parseInt(el.value, 10);
                if (!isNaN(v) && v >= 0) elementCounts[key] = v;
            }
        }

        settingsError.hidden = true;
        btnStartExpedition.disabled = true;
        btnStartExpedition.textContent = 'Building world…';
        const started = await startNewGame(
            rowsInput.value,
            columnsInput.value,
            difficultyInput.value,
            seedInput.value || null,
            elementCounts,
            aggressionInput.value,
            floorsInput.value,
        );
        btnStartExpedition.disabled = false;
        btnStartExpedition.textContent = 'Start expedition';
        if (started) {
            closeSettings();
        } else {
            settingsError.textContent = messageLog.textContent.replace(/^Error:\s*/, '');
            settingsError.hidden = false;
        }
    });
}

function isSettingsOpen() {
    return !modalBackdrop.classList.contains('hidden');
}

function openSettings() {
    if (currentGameState) {
        rowsInput.value = currentGameState.rows;
        columnsInput.value = currentGameState.columns;
        floorsInput.value = currentGameState.totalLevels;
        difficultyInput.value = currentGameState.difficulty;
        aggressionInput.value = currentGameState.enemyAggression;
        for (const [key, input] of Object.entries(countInputs)) {
            const count = currentGameState.elementCounts?.[key];
            input.value = Number.isInteger(count) ? count : '';
        }
    }
    updateSettingsPreview();
    settingsError.hidden = true;
    modalBackdrop.classList.remove('hidden');
    modalBackdrop.removeAttribute('inert');
    modalBackdrop.setAttribute('aria-hidden', 'false');
    document.body.classList.add('modal-open');
    btnCloseModal.focus();
}

function closeSettings() {
    modalBackdrop.classList.add('hidden');
    modalBackdrop.setAttribute('aria-hidden', 'true');
    modalBackdrop.setAttribute('inert', '');
    document.body.classList.remove('modal-open');
    btnCustomMap.focus();
}

function updateSettingsPreview() {
    const rows = Number(rowsInput.value) || 0;
    const columns = Number(columnsInput.value) || 0;
    const aggression = Number(aggressionInput.value);
    const floors = Number(floorsInput.value) || 0;
    const difficulty = difficultyInput.options[difficultyInput.selectedIndex]?.text.split(' · ')[0] || 'Normal';
    const customEntries = Object.entries(countInputs).filter(([, input]) => input.value !== '');
    const customTerrainCount = customEntries.length;
    const customTileCount = customEntries.reduce((total, [name, input]) => {
        const count = Number(input.value) || 0;
        return total + (name === 'portal' ? count * 2 : count);
    }, 0);
    const area = rows * columns;
    const validArea = area <= 200;

    mapSizePreview.textContent = `${area} / 200 tiles`;
    mapSizePreview.classList.toggle('error', !validArea);
    columnsInput.setCustomValidity(validArea ? '' : 'A floor can contain at most 200 tiles. Reduce rows or columns.');
    aggressionValue.textContent = `${aggression}%`;
    const typeLabel = customTerrainCount === 1 ? 'type' : 'types';
    const tileLabel = customTileCount === 1 ? 'tile' : 'tiles';
    terrainMode.textContent = customTerrainCount ? `${customTerrainCount} ${typeLabel} · ${customTileCount} ${tileLabel}` : 'Automatic';
    settingsSummary.textContent = `${rows} × ${columns} · ${floors} floor${floors === 1 ? '' : 's'} · ${difficulty} · ${aggression}% pursuit`;

    if (aggression <= 25) {
        aggressionDescription.textContent = 'Watchful · the bear will mostly patrol the map.';
    } else if (aggression <= 60) {
        aggressionDescription.textContent = 'Balanced · pursuit and patrol are equally likely.';
    } else if (aggression <= 85) {
        aggressionDescription.textContent = 'Aggressive · the bear usually hunts you or nearby prey.';
    } else {
        aggressionDescription.textContent = 'Relentless · expect almost constant pursuit.';
    }

    presetButtons.forEach((button) => {
        const active = Number(button.dataset.rows) === rows && Number(button.dataset.columns) === columns;
        button.classList.toggle('active', active);
        button.setAttribute('aria-pressed', String(active));
    });
}

async function responseError(response) {
    try {
        const body = await response.json();
        return body.error || `Request failed (${response.status})`;
    } catch {
        return `Request failed (${response.status})`;
    }
}
