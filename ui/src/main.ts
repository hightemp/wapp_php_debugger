import { mount } from 'svelte';
import App from './App.svelte';
import './styles/theme.css';
import './styles/controls.css';

const target = document.getElementById('app');
if (!target) {
  throw new Error('#app not found');
}

export default mount(App, { target });
