/**
 * Replay Theater & Playback Engine
 * Loads, plays, pauses, steps through, and seeks through recorded YGOPro match replays.
 */

import { WailsBridge, eventBus } from '../wails_bridge.js';

export class ReplayViewer {
  constructor(elements, onStartPlayback) {
    this.elements = elements;
    this.onStartPlayback = onStartPlayback;

    this.isPlaying = false;
    this.currentStep = 0;
    this.playbackSpeed = 1000; // ms per step
    this.timer = null;
    this.replayEvents = [];

    this.initEventListeners();
  }

  initEventListeners() {
    this.elements.playBtn.addEventListener('click', () => this.togglePlay());
    this.elements.stepForwardBtn.addEventListener('click', () => this.stepForward());
    this.elements.stepBackBtn.addEventListener('click', () => this.stepBack());
    this.elements.speedSelect.addEventListener('change', (e) => {
      const speedMult = parseFloat(e.target.value) || 1;
      this.playbackSpeed = 1000 / speedMult;
      if (this.isPlaying) {
        this.pause();
        this.play();
      }
    });
  }

  loadReplay(replayData) {
    this.replayEvents = replayData.events || [];
    this.currentStep = 0;
    this.elements.totalStepsLabel.innerText = this.replayEvents.length;
    this.elements.currentStepLabel.innerText = '0';
  }

  togglePlay() {
    if (this.isPlaying) this.pause();
    else this.play();
  }

  play() {
    this.isPlaying = true;
    this.elements.playBtn.innerText = '⏸ Pause';
    this.timer = setInterval(() => {
      if (this.currentStep < this.replayEvents.length) {
        this.stepForward();
      } else {
        this.pause();
      }
    }, this.playbackSpeed);
  }

  pause() {
    this.isPlaying = false;
    this.elements.playBtn.innerText = '▶ Play';
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  stepForward() {
    if (this.currentStep < this.replayEvents.length) {
      const evt = this.replayEvents[this.currentStep];
      eventBus.emit(evt.type, evt.data);
      this.currentStep++;
      this.elements.currentStepLabel.innerText = this.currentStep;
      this.elements.progressBar.style.width = `${(this.currentStep / this.replayEvents.length) * 100}%`;
    }
  }

  stepBack() {
    if (this.currentStep > 0) {
      this.currentStep--;
      this.elements.currentStepLabel.innerText = this.currentStep;
      this.elements.progressBar.style.width = `${(this.currentStep / this.replayEvents.length) * 100}%`;
    }
  }
}
