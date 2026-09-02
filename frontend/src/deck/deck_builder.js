/**
 * Visual Deck Builder System
 * Main, Extra, and Side deck editing, card search & filter, stats, YDK persistence.
 */

import { WailsBridge } from '../wails_bridge.js';

export class DeckBuilder {
  constructor(elements, onInspectCard) {
    this.elements = elements;
    this.onInspectCard = onInspectCard;

    this.currentDeck = {
      name: 'New Deck',
      main: [],
      extra: [],
      side: []
    };

    this.initEventListeners();
    this.loadDeckList();
  }

  initEventListeners() {
    // Search input
    this.elements.searchInput.addEventListener('input', () => this.performSearch());
    this.elements.typeFilter.addEventListener('change', () => this.performSearch());

    // Deck actions
    this.elements.saveDeckBtn.addEventListener('click', () => this.saveDeck());
    this.elements.newDeckBtn.addEventListener('click', () => this.newDeck());
    this.elements.deckSelect.addEventListener('change', (e) => this.loadDeck(e.target.value));

    // Sample hand test
    this.elements.sampleHandBtn.addEventListener('click', () => this.simulateSampleHand());
  }

  async loadDeckList() {
    const list = await WailsBridge.listDecks();
    this.elements.deckSelect.innerHTML = '';
    list.forEach(name => {
      const opt = document.createElement('option');
      opt.value = name;
      opt.innerText = name;
      this.elements.deckSelect.appendChild(opt);
    });

    if (list.length > 0) {
      this.loadDeck(list[0]);
    } else {
      this.renderDeck();
    }
  }

  async loadDeck(name) {
    const deck = await WailsBridge.loadDeck(name);
    if (deck) {
      this.currentDeck = deck;
      this.elements.deckNameInput.value = deck.name;
      this.renderDeck();
    }
  }

  async saveDeck() {
    this.currentDeck.name = this.elements.deckNameInput.value.trim() || 'Untitled Deck';
    await WailsBridge.saveDeck(this.currentDeck);
    alert(`Deck "${this.currentDeck.name}" saved successfully!`);
    this.loadDeckList();
  }

  newDeck() {
    this.currentDeck = {
      name: 'New Deck',
      main: [],
      extra: [],
      side: []
    };
    this.elements.deckNameInput.value = 'New Deck';
    this.renderDeck();
  }

  async performSearch() {
    const keyword = this.elements.searchInput.value.trim();
    const typeVal = parseInt(this.elements.typeFilter.value, 10) || 0;

    const results = await WailsBridge.searchCards({
      keyword,
      type: typeVal,
      limit: 40
    });

    this.renderSearchResults(results);
  }

  renderSearchResults(cards) {
    const container = this.elements.searchResultsGrid;
    container.innerHTML = '';

    cards.forEach(card => {
      const item = document.createElement('div');
      item.className = 'select-card-item';
      item.style.background = '#1e293b';
      item.style.padding = '6px';
      item.innerHTML = `
        <div style="font-size:11px; font-weight:700; color:#38bdf8; overflow:hidden; text-overflow:ellipsis;">${card.name}</div>
        <div style="font-size:10px; color:#94a3b8; margin-top:4px;">${card.attack !== undefined ? 'ATK/' + card.attack : ''}</div>
      `;

      item.addEventListener('mouseenter', () => {
        if (this.onInspectCard) this.onInspectCard(card.code, card);
      });

      item.addEventListener('click', () => {
        this.addCardToDeck(card);
      });

      container.appendChild(item);
    });
  }

  addCardToDeck(card) {
    const isExtra = (card.type & (0x40 | 0x2000 | 0x800000 | 0x4000000)) !== 0;

    if (isExtra) {
      if (this.currentDeck.extra.length < 15) {
        this.currentDeck.extra.push(card.code);
      } else {
        alert('Extra deck is full (15 cards max).');
      }
    } else {
      if (this.currentDeck.main.length < 60) {
        this.currentDeck.main.push(card.code);
      } else {
        alert('Main deck is full (60 cards max).');
      }
    }
    this.renderDeck();
  }

  removeCardFromDeck(section, index) {
    if (section === 'main') this.currentDeck.main.splice(index, 1);
    else if (section === 'extra') this.currentDeck.extra.splice(index, 1);
    else if (section === 'side') this.currentDeck.side.splice(index, 1);
    this.renderDeck();
  }

  async renderDeck() {
    const renderSection = async (codes, containerElem, sectionName) => {
      containerElem.innerHTML = '';
      for (let i = 0; i < codes.length; i++) {
        const code = codes[i];
        const card = await WailsBridge.getCard(code);
        const item = document.createElement('div');
        item.className = 'select-card-item';
        item.style.background = '#0f172a';
        item.style.padding = '4px';
        item.style.height = '100px';
        item.innerHTML = `
          <div style="font-size:10px; font-weight:700; color:#38bdf8; overflow:hidden; text-overflow:ellipsis;">${card ? card.name : code}</div>
        `;

        item.addEventListener('mouseenter', () => {
          if (this.onInspectCard && card) this.onInspectCard(code, card);
        });

        item.addEventListener('contextmenu', (e) => {
          e.preventDefault();
          this.removeCardFromDeck(sectionName, i);
        });

        item.addEventListener('click', () => {
          this.removeCardFromDeck(sectionName, i);
        });

        containerElem.appendChild(item);
      }
    };

    await renderSection(this.currentDeck.main, this.elements.mainDeckGrid, 'main');
    await renderSection(this.currentDeck.extra, this.elements.extraDeckGrid, 'extra');
    await renderSection(this.currentDeck.side, this.elements.sideDeckGrid, 'side');

    this.elements.mainCountBadge.innerText = `${this.currentDeck.main.length} / 60`;
    this.elements.extraCountBadge.innerText = `${this.currentDeck.extra.length} / 15`;
    this.elements.sideCountBadge.innerText = `${this.currentDeck.side.length} / 15`;
  }

  simulateSampleHand() {
    if (this.currentDeck.main.length < 5) {
      alert('You need at least 5 cards in your Main Deck to test a sample hand.');
      return;
    }
    const shuffled = [...this.currentDeck.main].sort(() => 0.5 - Math.random());
    const hand = shuffled.slice(0, 5);
    alert(`Sample 5-Card Opening Hand Drawn:\n` + hand.join('\n'));
  }
}
