import { mount } from 'svelte'
import App from './App.svelte'
import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans/latin-500.css'
import '@fontsource/ibm-plex-sans/latin-600.css'
import '@fontsource/ibm-plex-mono/latin-500.css'
import './vpro.css'

mount(App, { target: document.getElementById('app')! })
