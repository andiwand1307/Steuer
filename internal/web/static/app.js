'use strict';

// ---------- Hilfsfunktionen ----------

const state = { jahr: 2026, meta: null };

function el(tag, props, ...kids) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v === undefined || v === null || v === false) continue;
    if (k === 'class') e.className = v;
    else if (k === 'text') e.textContent = v;
    else if (k.startsWith('on')) e.addEventListener(k.slice(2), v);
    else if (['value', 'checked', 'disabled', 'selected', 'open'].includes(k)) e[k] = v;
    else e.setAttribute(k, v === true ? '' : v);
  }
  for (const kid of kids.flat(Infinity)) {
    if (kid === null || kid === undefined || kid === false) continue;
    e.append(kid.nodeType ? kid : document.createTextNode(String(kid)));
  }
  return e;
}

const fmtEuro = new Intl.NumberFormat('de-DE', { style: 'currency', currency: 'EUR' });
function euro(cent) { return fmtEuro.format((cent || 0) / 100); }

// Deutsche Zahleneingabe -> Cent (ganze Zahl) oder null.
function parseBetrag(eingabe) {
  let s = String(eingabe).replace(/[\s €]|EUR/gi, '');
  if (!s) return null;
  let neg = false;
  if (s[0] === '-') { neg = true; s = s.slice(1); } else if (s[0] === '+') { s = s.slice(1); }
  if (s.includes(',')) {
    if ((s.match(/,/g) || []).length > 1) return null;
    s = s.replace(/\./g, '');
  } else if ((s.match(/\./g) || []).length > 1) {
    s = s.replace(/\./g, '');
  } else {
    const i = s.lastIndexOf('.');
    if (i >= 0 && s.length - i - 1 === 3) s = s.replace('.', '');
  }
  const m = /^(\d+)(?:[.,](\d{1,2}))?$/.exec(s.replace(',', '.'));
  if (!m) return null;
  const c = parseInt(m[1], 10) * 100 + (m[2] ? parseInt(m[2].padEnd(2, '0'), 10) : 0);
  return neg ? -c : c;
}

// Cent -> Text für Eingabefelder ("1.234,56")
function betragText(cent) {
  if (cent === null || cent === undefined) return '';
  return new Intl.NumberFormat('de-DE', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(cent / 100);
}

function isoZuDe(iso) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso || '');
  return m ? `${m[3]}.${m[2]}.${m[1]}` : (iso || '');
}

// "1234.5" -> "1234,5"
function zahlText(s) { return String(s || '').replace('.', ','); }

function prozent(p) {
  return (p > 0 ? '+' : '') + new Intl.NumberFormat('de-DE', { maximumFractionDigits: 0 }).format(p) + ' %';
}

function personName(p) { return p === 'A' ? 'Person A' : p === 'B' ? 'Person B' : 'gemeinsam'; }

function meldung(text, istFehler) {
  const box = document.getElementById('meldung');
  leer(box);
  const t = el('div', { class: 'toast' + (istFehler ? ' fehler' : ''), text, onclick: () => t.remove() });
  if (istFehler) t.setAttribute('role', 'alert');
  box.append(t);
  setTimeout(() => t.remove(), istFehler ? 8000 : 2500);
}

async function api(methode, url, body) {
  const opt = { method: methode, headers: {} };
  if (methode !== 'GET') {
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body === undefined ? {} : body);
  }
  let r;
  try {
    r = await fetch(url, opt);
  } catch (e) {
    throw new Error('Das Programm antwortet nicht. Läuft es noch?');
  }
  let daten = null;
  try { daten = await r.json(); } catch (e) { /* kein JSON */ }
  if (!r.ok) throw new Error((daten && daten.fehler) || ('Fehler ' + r.status));
  return daten;
}

// Führt eine Aktion aus und zeigt Fehler als Meldung an.
async function tu(fn) {
  try { return await fn(); } catch (e) { meldung(e.message, true); return undefined; }
}

function leer(node) { while (node.firstChild) node.removeChild(node.firstChild); return node; }

function jahrQuery() { return '?jahr=' + state.jahr; }

function kategorieTitel(k) {
  const c = state.meta.kategorien.find((x) => x.schluessel === k);
  return c ? c.titel : k;
}

function kategorieSelect(wert, props) {
  const sel = el('select', props);
  const gruppen = [];
  for (const k of state.meta.kategorien) {
    let g = gruppen.find((x) => x.name === k.gruppe);
    if (!g) { g = { name: k.gruppe, kinder: [] }; gruppen.push(g); }
    g.kinder.push(k);
  }
  for (const g of gruppen) {
    sel.append(el('optgroup', { label: g.name }, g.kinder.map((k) => el('option', { value: k.schluessel, text: k.titel }))));
  }
  sel.value = wert || state.meta.kategorien[0].schluessel;
  return sel;
}

function personSelect(wert, props) {
  const sel = el('select', props,
    el('option', { value: '', text: 'gemeinsam' }),
    el('option', { value: 'A', text: 'Person A' }),
    el('option', { value: 'B', text: 'Person B' }));
  sel.value = wert || '';
  return sel;
}


// Beschriftung muss das Feld umschließen: einfacher Aufbau mit label > text + input
function beschriftet(label, input, klasse) {
  return el('label', { class: 'feld ' + (klasse || '') }, el('span', { text: label }), input);
}

function marke(klasse, text) { return el('span', { class: 'marke ' + klasse, text }); }

function sicherheitsMarke(s) {
  if (s === 'sicher') return marke('sicher', '✓ sicher');
  if (s === 'stark evident') return marke('stark', '≈ stark evident');
  if (s === 'unsicher') return marke('unsicher', '⚠ unsicher');
  return el('span', { class: 'leise', text: s || '–' });
}

function statusMarke(s) {
  const t = { neu: 'neu – ungeprüft', bestaetigt: 'bestätigt', verworfen: 'verworfen' };
  return marke(s, t[s] || s);
}

const stufenText = { warnung: 'Warnung', pruefen: 'Bitte prüfen', info: 'Information' };
function hinweisBox(h) {
  return el('div', { class: 'hinweis ' + h.stufe },
    el('span', { class: 'stufe', text: (h.stufe === 'warnung' ? '⚠ ' : h.stufe === 'pruefen' ? '? ' : 'i ') + (stufenText[h.stufe] || h.stufe) + ':' }),
    el('strong', { text: h.thema ? h.thema + '. ' : '' }), h.text);
}

function kopf(titel, ...rest) { return el('div', { class: 'leiste' }, el('h2', { text: titel, class: 'ohne-rand' }), ...rest); }

function merkeScroll(fn) {
  return async (...a) => { const y = window.scrollY; await fn(...a); window.scrollTo(0, y); };
}

// ---------- Router ----------

const seiten = {};
let aktuell = 0;

async function zeige() {
  const teile = (location.hash || '#/uebersicht').replace(/^#\//, '').split('/');
  const name = teile[0] in seiten ? teile[0] : 'uebersicht';
  for (const a of document.querySelectorAll('#nav a')) {
    if (a.dataset.seite === name) a.setAttribute('aria-current', 'page'); else a.removeAttribute('aria-current');
  }
  const main = document.getElementById('inhalt');
  const lauf = ++aktuell;
  try {
    const knoten = await seiten[name](teile.slice(1));
    if (lauf !== aktuell) return;
    leer(main).append(knoten);
  } catch (e) {
    if (lauf !== aktuell) return;
    leer(main).append(el('div', { class: 'fehlerbox', role: 'alert' }, el('strong', { text: 'Das hat nicht geklappt. ' }), e.message));
  }
}

async function neuLaden() {
  const y = window.scrollY;
  await zeige();
  window.scrollTo(0, y);
}

// ---------- Übersicht ----------

seiten.uebersicht = async () => {
  const u = await api('GET', '/api/uebersicht' + jahrQuery());
  const ergebnis = el('div');
  if (!u.berechnet) {
    ergebnis.append(el('div', { class: 'fehlerbox' }, el('strong', { text: 'Noch keine Berechnung möglich. ' }), u.fehler,
      el('p', {}, el('a', { href: '#/angaben', text: 'Zu den Angaben' }))));
  } else {
    const erst = u.erstattung_cent;
    ergebnis.append(el('div', { class: 'karten' },
      el('div', { class: 'karte ' + (erst >= 0 ? 'erstattung' : 'nachzahlung') },
        el('span', { class: 'beschr', text: erst >= 0 ? 'Erwartete Erstattung' : 'Erwartete Nachzahlung' }),
        el('span', { class: 'zahl', text: euro(Math.abs(erst)) }),
        el('span', { class: 'beschr', text: 'nach Abzug aller Vorauszahlungen und der Lohnsteuer' })),
      el('div', { class: 'karte' }, el('span', { class: 'beschr', text: 'Zu versteuerndes Einkommen' }), el('span', { class: 'zahl', text: euro(u.zve_cent) })),
      el('div', { class: 'karte' }, el('span', { class: 'beschr', text: 'Einkommensteuer' }), el('span', { class: 'zahl', text: euro(u.est_cent) }),
        el('span', { class: 'beschr', text: 'dazu Solidaritätszuschlag ' + euro(u.soli_cent) }))));
  }
  const status = el('div', { class: 'box' },
    el('h3', { text: 'Stand der Erfassung' }),
    el('ul', { class: 'punkte' },
      el('li', {}, el('strong', { text: u.belege_neu + ' ' }), u.belege_neu === 1 ? 'Beleg noch ungeprüft ' : 'Belege noch ungeprüft ', el('a', { href: '#/belege', text: '(ansehen)' }),
        u.belege_neu_andere > 0 ? ' – dazu ' + u.belege_neu_andere + ' in anderen Steuerjahren' : ''),
      el('li', {}, el('strong', { text: u.vorschlaege_offen + ' ' }), 'unbestätigte Positionen (gehen noch nicht in die Berechnung ein)'),
      el('li', {}, el('strong', { text: u.hinweise_offen + ' ' }), 'offene Hinweise zum Prüfen' + (u.hinweise_warnungen ? ', davon ' + u.hinweise_warnungen + ' Warnung(en) ' : ' '), el('a', { href: '#/erklaerung', text: '(ansehen)' }))));
  const ergebnisInfo = el('p', { class: 'leise' });
  const knopf = el('button', { class: 'haupt', text: 'Eingang jetzt prüfen' });
  knopf.addEventListener('click', async () => {
    knopf.disabled = true;
    const r = await tu(() => api('POST', '/api/eingang/pruefen'));
    knopf.disabled = false;
    if (!r) return;
    let t = r.neu === 0 ? 'Keine neuen Dateien gefunden.' : r.neu + ' neue(r) Beleg(e) erfasst.';
    if (r.fehler.length) t += ' Probleme: ' + r.fehler.join('; ');
    meldung(t, r.fehler.length > 0);
    await neuLaden();
  });
  const eingang = el('div', { class: 'box' },
    el('h3', { text: 'Eingangsordner' }),
    el('p', {}, 'Lege Rechnungen, Bescheinigungen und Bescheide (PDF, Fotos, Text) in diesen Ordner. Das Programm prüft ihn alle paar Sekunden von selbst:'),
    el('p', {}, el('code', { text: u.eingang, class: 'pfad' })),
    el('p', {}, 'Texterkennung für Bilder und Scans: ', u.texterkennung
      ? marke('sicher', '✓ verfügbar (Tesseract gefunden)')
      : marke('stark', 'nicht installiert – PDFs mit Text und Textdateien funktionieren trotzdem; Scans und Fotos nicht')),
    el('div', { class: 'leiste' }, knopf));
  return el('div', {}, el('h2', { text: 'Übersicht Steuerjahr ' + u.jahr }), ergebnis, status, eingang, ergebnisInfo);
};

// ---------- Belege ----------

const filter = { jahr: 'aktuell', status: 'alle' };

seiten.belege = async (param) => {
  if (param[0]) return belegDetail(parseInt(param[0], 10));
  const q = (filter.jahr === 'alle' ? 'alle' : state.jahr);
  const d = await api('GET', '/api/belege?jahr=' + q + '&status=' + filter.status);
  const selJahr = el('select', { 'aria-label': 'Steuerjahr der Belege', onchange: (e) => { filter.jahr = e.target.value; zeige(); } },
    el('option', { value: 'aktuell', text: 'Steuerjahr ' + state.jahr }), el('option', { value: 'alle', text: 'alle Steuerjahre' }));
  selJahr.value = filter.jahr;
  const selStatus = el('select', { 'aria-label': 'Status', onchange: (e) => { filter.status = e.target.value; zeige(); } },
    ...[['alle', 'alle Status'], ['neu', 'neu (ungeprüft)'], ['bestaetigt', 'bestätigt'], ['verworfen', 'verworfen']].map(([v, t]) => el('option', { value: v, text: t })));
  selStatus.value = filter.status;
  let inhalt;
  if (!d.belege.length) {
    inhalt = el('p', { class: 'box', text: 'Keine Belege für diese Auswahl. Lege Dateien in den Eingangsordner (siehe Übersicht).' });
  } else {
    inhalt = el('div', { class: 'tabelle-rahmen' }, el('table', {},
      el('thead', {}, el('tr', {}, ['Nr.', 'Status', 'Art', 'Datum', 'Aussteller', 'Betrag', 'Jahr', 'Vorschläge', ''].map((t, i) => el('th', { class: i === 5 ? 'betrag' : '', text: t })))),
      el('tbody', {}, d.belege.map((b) => el('tr', {},
        el('td', { text: b.id }),
        el('td', {}, statusMarke(b.status)),
        el('td', { text: b.art }),
        el('td', { text: isoZuDe(b.belegdatum) }),
        el('td', { text: b.aussteller }),
        el('td', { class: 'betrag', text: b.betrag_cent ? euro(b.betrag_cent) : '–' }),
        el('td', { text: b.steuerjahr }),
        el('td', { text: b.offene_vorschlaege ? b.offene_vorschlaege + ' offen' : (b.anzahl_positionen ? 'alle bestätigt' : '–') }),
        el('td', {}, el('a', { class: 'knopf', href: '#/belege/' + b.id, text: 'Öffnen' })))))));
  }
  return el('div', {}, el('h2', { text: 'Belege' }), el('div', { class: 'leiste' }, selJahr, selStatus), inhalt);
};

async function belegDetail(id) {
  const d = await api('GET', '/api/belege/' + id);
  const b = d.beleg;
  const art = el('input', { type: 'text', value: b.art, list: 'belegarten', 'aria-label': 'Art' });
  const liste = el('datalist', { id: 'belegarten' }, state.meta.belegarten.map((a) => el('option', { value: a })));
  const datum = el('input', { type: 'text', value: isoZuDe(b.belegdatum), placeholder: 'TT.MM.JJJJ' });
  const aussteller = el('input', { type: 'text', value: b.aussteller });
  const betrag = el('input', { type: 'text', value: b.betrag_cent ? betragText(b.betrag_cent) : '', inputmode: 'decimal', placeholder: '0,00' });
  const jahr = el('input', { type: 'text', value: b.steuerjahr, inputmode: 'numeric' });
  const speichern = async (status) => {
    const cent = betrag.value.trim() === '' ? 0 : parseBetrag(betrag.value);
    if (cent === null) { meldung('Der Betrag ist keine gültige Zahl (z. B. 1.234,56).', true); return; }
    const r = await tu(() => api('PUT', '/api/belege/' + id, {
      steuerjahr: parseInt(jahr.value, 10) || 0, belegdatum: datum.value, aussteller: aussteller.value,
      art: art.value, betrag_cent: cent, status: status || b.status,
    }));
    if (r) { meldung('Beleg gespeichert.'); await neuLaden(); }
  };
  const kopfTeil = el('div', { class: 'box' },
    el('div', { class: 'leiste' }, el('h3', { text: 'Beleg ' + b.id, class: 'ohne-rand' }), statusMarke(b.status),
      el('a', { class: 'knopf rechts', href: '#/belege', text: '← zur Liste' })),
    b.hinweis ? el('div', { class: 'hinweis pruefen' }, el('strong', { text: 'Hinweis der Erkennung: ' }), b.hinweis) : null,
    el('div', { class: 'gitter' },
      beschriftet('Art des Belegs', art), beschriftet('Belegdatum', datum), beschriftet('Aussteller', aussteller),
      beschriftet('Gesamtbetrag (€)', betrag), beschriftet('Steuerjahr', jahr)),
    liste,
    el('div', { class: 'leiste' },
      el('button', { class: 'haupt', text: 'Speichern', onclick: () => speichern() }),
      el('button', { text: 'Beleg bestätigen', onclick: () => speichern('bestaetigt') }),
      el('button', { text: 'Als nicht relevant verwerfen', onclick: () => speichern('verworfen') }),
      b.status !== 'neu' ? el('button', { text: 'Wieder auf „neu“ setzen', onclick: () => speichern('neu') }) : null,
      el('a', { class: 'knopf', href: '/api/belege/' + id + '/datei', target: '_blank', rel: 'noopener', text: 'Datei öffnen' }),
      el('button', {
        class: 'gefahr rechts', text: 'Beleg aus der Liste löschen', onclick: async () => {
          if (!confirm('Beleg und seine unbestätigten Vorschläge aus dem Programm löschen? Die Datei bleibt im Ordner Belege erhalten.')) return;
          const r = await tu(() => api('DELETE', '/api/belege/' + id));
          if (r) { meldung('Beleg gelöscht.'); location.hash = '#/belege'; }
        },
      })),
    el('p', { class: 'leise', text: 'Datei: ' + b.datei }));

  const vorschlaege = d.positionen.filter((p) => !p.bestaetigt);
  const bulkPerson = el('select', { 'aria-label': 'Person für alle Vorschläge' }, el('option', { value: '?', text: 'je Position wie eingestellt' }),
    el('option', { value: 'A', text: 'Person A' }), el('option', { value: 'B', text: 'Person B' }), el('option', { value: '', text: 'gemeinsam' }));
  const posBox = el('div', {}, el('h3', { text: 'Positionen aus diesem Beleg' }),
    d.positionen.length ? null : el('p', { class: 'leise', text: 'Aus diesem Beleg wurde nichts vorgeschlagen. Du kannst unten eine Position ergänzen.' }),
    vorschlaege.length > 1 ? el('div', { class: 'leiste' }, el('label', {}, 'Person für alle Vorschläge: ', bulkPerson), el('button', {
      class: 'haupt', text: 'Alle ' + vorschlaege.length + ' Vorschläge bestätigen', onclick: async () => {
        for (const p of vorschlaege) {
          const ue = { bestaetigt: true };
          if (bulkPerson.value !== '?') ue.person = bulkPerson.value;
          const r = await tu(() => api('PUT', '/api/positionen/' + p.id, posBody(p, ue)));
          if (!r) break;
        }
        await neuLaden();
      },
    })) : null,
    d.positionen.map((p) => positionsEditor(p, { onChange: neuLaden })),
    el('h3', { text: 'Position ergänzen' }),
    positionsEditor({ id: 0, steuerjahr: b.steuerjahr, beleg_id: id, person: '', kategorie: '', bezeichnung: '', betrag_cent: 0, datum: b.belegdatum, notiz: '', bestaetigt: false }, { onChange: neuLaden, neu: true }));

  const text = el('details', {}, el('summary', { text: 'Erkannter Text anzeigen' }),
    el('pre', { class: 'beleg-text', text: b.text || '(kein Text erkannt)' }));
  return el('div', {}, kopfTeil, text, posBox);
}

// ---------- Positionen (Editor) ----------

function posBody(p, ueberschreibe) {
  return Object.assign({
    steuerjahr: p.steuerjahr, beleg_id: p.beleg_id, person: p.person, kategorie: p.kategorie,
    bezeichnung: p.bezeichnung, betrag_cent: p.betrag_cent, datum: p.datum, notiz: p.notiz, bestaetigt: p.bestaetigt,
  }, ueberschreibe || {});
}

function zuordnungsText(kat) {
  const z = state.meta.zuordnungen[kat];
  if (!z) return '';
  let t = 'Landet in: ' + z.formular;
  if (z.zeile) t += ', Zeile ' + z.zeile;
  if (z.kennziffer) t += ' (Kennziffer ' + z.kennziffer + ')';
  if (z.hinweis) t += ' – ' + z.hinweis;
  return t;
}

function positionsEditor(p, opt) {
  const kat = kategorieSelect(p.kategorie, { 'aria-label': 'Kategorie' });
  const person = personSelect(p.person, { 'aria-label': 'Person' });
  const bez = el('input', { type: 'text', value: p.bezeichnung, 'aria-label': 'Bezeichnung' });
  const betrag = el('input', { type: 'text', value: p.id || p.betrag_cent ? betragText(p.betrag_cent) : '', inputmode: 'decimal', placeholder: '0,00', 'aria-label': 'Betrag in Euro' });
  const datum = el('input', { type: 'text', value: isoZuDe(p.datum), placeholder: 'TT.MM.JJJJ', 'aria-label': 'Datum' });
  const notiz = el('input', { type: 'text', value: p.notiz, 'aria-label': 'Notiz' });
  const zeile = el('p', { class: 'zeile', text: zuordnungsText(kat.value) });
  kat.addEventListener('change', () => { zeile.textContent = zuordnungsText(kat.value); });

  const sammeln = (bestaetigt) => {
    const cent = parseBetrag(betrag.value);
    if (cent === null) { meldung('Der Betrag ist keine gültige Zahl (z. B. 1.234,56).', true); betrag.focus(); return null; }
    return posBody(p, { person: person.value, kategorie: kat.value, bezeichnung: bez.value, betrag_cent: cent, datum: datum.value, notiz: notiz.value, bestaetigt });
  };
  const speichern = async (bestaetigt, text) => {
    const body = sammeln(bestaetigt);
    if (!body) return;
    const r = await tu(() => (p.id ? api('PUT', '/api/positionen/' + p.id, body) : api('POST', '/api/positionen', body)));
    if (r) { meldung(text); await opt.onChange(); }
  };
  const knoepfe = el('div', { class: 'knoepfe' });
  if (opt.neu) {
    knoepfe.append(el('button', { class: 'haupt', text: 'Hinzufügen und bestätigen', onclick: () => speichern(true, 'Position hinzugefügt.') }));
  } else {
    if (!p.bestaetigt) knoepfe.append(el('button', { class: 'haupt', text: 'Bestätigen', onclick: () => speichern(true, 'Position bestätigt.') }));
    knoepfe.append(el('button', { text: 'Änderungen speichern', onclick: () => speichern(p.bestaetigt, 'Position gespeichert.') }));
    if (p.bestaetigt) knoepfe.append(el('button', { text: 'Auf „unbestätigt“ zurücksetzen', onclick: () => speichern(false, 'Position ist jetzt unbestätigt.') }));
    knoepfe.append(el('button', {
      class: 'gefahr', text: p.bestaetigt ? 'Löschen' : 'Verwerfen', onclick: async () => {
        if (!confirm('Diese Position wirklich ' + (p.bestaetigt ? 'löschen' : 'verwerfen') + '?')) return;
        const r = await tu(() => api('DELETE', '/api/positionen/' + p.id));
        if (r) { meldung('Position entfernt.'); await opt.onChange(); }
      },
    }));
  }
  return el('div', { class: 'posform' + (!p.bestaetigt && !opt.neu ? ' vorschlag' : '') },
    !opt.neu ? el('div', { class: 'leiste' }, p.bestaetigt ? marke('bestaetigt', 'bestätigt') : marke('neu', 'Vorschlag – noch nicht in der Berechnung'),
      p.beleg_id ? el('a', { href: '#/belege/' + p.beleg_id, text: 'Beleg ' + p.beleg_id }) : el('span', { class: 'leise', text: 'ohne Beleg' })) : null,
    el('div', { class: 'gitter' },
      beschriftet('Kategorie', kat, 'breit'), beschriftet('Person', person), beschriftet('Betrag (€)', betrag),
      beschriftet('Bezeichnung', bez), beschriftet('Datum', datum), beschriftet('Notiz', notiz, 'breit')),
    zeile, knoepfe);
}

seiten.positionen = async () => {
  const d = await api('GET', '/api/positionen' + jahrQuery());
  const neu = positionsEditor({ id: 0, steuerjahr: state.jahr, beleg_id: null, person: '', kategorie: '', bezeichnung: '', betrag_cent: 0, datum: '', notiz: '', bestaetigt: true }, { onChange: neuLaden, neu: true });
  const neuBox = el('details', { class: 'box' }, el('summary', { text: 'Position manuell hinzufügen' }), neu);
  // gruppieren
  const gruppen = [];
  for (const k of state.meta.kategorien) {
    const ps = d.positionen.filter((p) => p.kategorie === k.schluessel);
    if (!ps.length) continue;
    let g = gruppen.find((x) => x.name === k.gruppe);
    if (!g) { g = { name: k.gruppe, kats: [] }; gruppen.push(g); }
    g.kats.push({ k, ps });
  }
  const inhalt = el('div');
  if (!gruppen.length) inhalt.append(el('p', { class: 'box', text: 'Für dieses Jahr gibt es noch keine Positionen.' }));
  for (const g of gruppen) {
    inhalt.append(el('h3', { text: g.name }));
    for (const { k, ps } of g.kats) {
      const bestSumme = ps.filter((p) => p.bestaetigt).reduce((a, p) => a + p.betrag_cent, 0);
      const offenSumme = ps.filter((p) => !p.bestaetigt).reduce((a, p) => a + p.betrag_cent, 0);
      inhalt.append(el('details', { class: 'box', open: true },
        el('summary', {}, k.titel + ' – Summe bestätigt: ', el('strong', { text: euro(bestSumme) }),
          offenSumme ? el('span', { class: 'leise', text: ' (zusätzlich unbestätigt: ' + euro(offenSumme) + ')' }) : null),
        ps.map((p) => positionsEditor(p, { onChange: neuLaden }))));
    }
  }
  return el('div', {}, el('h2', { text: 'Positionen Steuerjahr ' + d.jahr }),
    el('p', { class: 'leise', text: 'Nur bestätigte Positionen gehen in die Berechnung ein.' }), neuBox, inhalt);
};

// ---------- Angaben ----------

function angabeAnzeige(def, wert) {
  if (!wert) return '';
  if (def.typ === 'datum') return isoZuDe(wert);
  if (def.typ === 'zahl') return zahlText(wert);
  if (def.typ === 'betrag') return betragText(Math.round(parseFloat(wert) * 100));
  return wert;
}

seiten.angaben = async () => {
  const d = await api('GET', '/api/angaben' + jahrQuery());
  if (!d.defs.length) {
    return el('div', {}, el('h2', { text: 'Angaben' }), el('div', { class: 'fehlerbox', text: 'Für das Steuerjahr ' + d.jahr + ' sind keine Steuerwerte hinterlegt. Wähle oben ein unterstütztes Jahr (aktuell 2026).' }));
  }
  const wurzel = el('div', {}, el('h2', { text: 'Angaben Steuerjahr ' + d.jahr }),
    el('p', { class: 'leise', text: 'Jedes Feld speicherst Du einzeln mit „Speichern“ (oder der Eingabetaste). Leer lassen ist erlaubt; dann gilt der Standardwert.' }));
  const gruppen = [];
  for (const def of d.defs) {
    let g = gruppen.find((x) => x.name === def.gruppe);
    if (!g) { g = { name: def.gruppe, defs: [] }; gruppen.push(g); }
    g.defs.push(def);
  }
  for (const g of gruppen) {
    const gesetzt = g.defs.filter((x) => (d.werte[x.schluessel] || '').trim() !== '').length;
    const sektion = el('details', { class: 'box', open: gesetzt === 0 || g.defs.length <= 6 || undefined },
      el('summary', {}, g.name, el('span', { class: 'leise', text: ' – ' + gesetzt + ' von ' + g.defs.length + ' ausgefüllt' })));
    for (const def of g.defs) sektion.append(angabeFeld(def, d.werte[def.schluessel] || '', d.jahr));
    wurzel.append(sektion);
  }
  return wurzel;
};

function angabeFeld(def, wert, jahr) {
  const id = 'ang-' + def.schluessel.replace(/\W/g, '_');
  let eingabe;
  if (def.typ === 'jaNein' || def.typ === 'auswahl') {
    const opts = def.typ === 'jaNein' ? ['ja', 'nein'] : def.optionen;
    eingabe = el('select', { id }, el('option', { value: '', text: '– nicht angegeben –' }), opts.map((o) => el('option', { value: o, text: o })));
    eingabe.value = wert;
  } else {
    eingabe = el('input', {
      id, type: 'text', value: angabeAnzeige(def, wert),
      inputmode: def.typ === 'zahl' || def.typ === 'betrag' ? 'decimal' : 'text',
      placeholder: def.typ === 'datum' ? 'TT.MM.JJJJ' : def.typ === 'betrag' ? '0,00' : '',
    });
  }
  const status = el('span', { class: 'gespeichert', 'aria-live': 'polite' });
  const speichern = async () => {
    status.textContent = '';
    const r = await tu(() => api('PUT', '/api/angaben', { jahr, schluessel: def.schluessel, wert: eingabe.value }));
    if (r) {
      if (eingabe.tagName === 'INPUT') eingabe.value = angabeAnzeige(def, r.wert);
      status.textContent = '✓ gespeichert';
    }
  };
  eingabe.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); speichern(); } });
  eingabe.addEventListener('input', () => { status.textContent = ''; });
  return el('div', { class: 'angabe' },
    el('label', { for: id, text: def.titel + (def.typ === 'betrag' ? ' (€)' : def.typ === 'datum' ? ' (TT.MM.JJJJ)' : '') }),
    def.hilfe ? el('p', { class: 'hilfe', text: def.hilfe }) : null,
    el('div', { class: 'eingabe' }, eingabe, el('button', { text: 'Speichern', onclick: speichern }), status));
}

// ---------- Erklärung ----------

function betragOderWert(z) {
  if (z.wert) {
    return el('span', {}, el('strong', { text: z.wert }), z.betrag_cent ? el('span', { class: 'leise', text: ' (daraus: ' + euro(z.betrag_cent) + ')' }) : null);
  }
  return el('strong', { text: euro(z.betrag_cent) });
}

seiten.erklaerung = async () => {
  const d = await api('GET', '/api/erklaerung' + jahrQuery());
  if (d.fehler) {
    return el('div', {}, el('h2', { text: 'Erklärung' }), el('div', { class: 'fehlerbox', role: 'alert' },
      el('strong', { text: 'Die Berechnung ist nicht möglich. ' }), d.fehler,
      el('p', {}, 'Korrigiere die genannten Angaben unter ', el('a', { href: '#/angaben', text: 'Angaben' }), '.')));
  }
  const erst = d.erstattung_cent;
  const wurzel = el('div', {}, el('h2', { text: 'Was trage ich wo ein? – Steuerjahr ' + d.jahr }),
    el('div', { class: 'leiste nicht-drucken' },
      el('button', { text: 'Drucken', onclick: () => window.print() }),
      el('a', { class: 'knopf', href: '/api/erklaerung.csv' + jahrQuery(), download: 'erklaerung-' + d.jahr + '.csv', text: 'Als CSV exportieren' })),
    el('div', { class: 'karten' },
      el('div', { class: 'karte ' + (erst >= 0 ? 'erstattung' : 'nachzahlung') }, el('span', { class: 'beschr', text: erst >= 0 ? 'Erwartete Erstattung' : 'Erwartete Nachzahlung' }), el('span', { class: 'zahl', text: euro(Math.abs(erst)) })),
      el('div', { class: 'karte' }, el('span', { class: 'beschr', text: 'Zu versteuerndes Einkommen' }), el('span', { class: 'zahl', text: euro(d.zve_cent) })),
      el('div', { class: 'karte' }, el('span', { class: 'beschr', text: 'Einkommensteuer + Soli' }), el('span', { class: 'zahl', text: euro(d.est_cent + d.soli_cent) }))));

  if (d.hinweise.length) {
    wurzel.append(el('h3', { text: 'Hinweise (' + d.hinweise.length + ')' }), ...d.hinweise.map(hinweisBox));
  }
  wurzel.append(el('h3', { text: 'Einzutragende Werte' }),
    el('p', { class: 'leise', text: 'Sicherheit: ✓ sicher = gesetzlich eindeutig; ≈ stark evident = sehr gut begründet; ⚠ unsicher = Auslegungsfrage, bitte prüfen. Klicke auf eine Bezeichnung für Herleitung und Rechtsgrundlage.' }));
  for (const f of d.formulare) {
    const mitPerson = f.zeilen.some((z) => z.person);
    wurzel.append(el('h3', { text: f.name }),
      el('div', { class: 'tabelle-rahmen' }, el('table', {},
        el('thead', {}, el('tr', {}, el('th', { text: 'Zeile' }), el('th', { text: 'Kennziffer' }), mitPerson ? el('th', { text: 'Person' }) : null,
          el('th', { text: 'Bezeichnung' }), el('th', { class: 'betrag', text: 'Eintragen' }), el('th', { text: 'Sicherheit' }))),
        el('tbody', {}, f.zeilen.map((z) => el('tr', {},
          el('td', { text: z.zeile }), el('td', { text: z.kennziffer }),
          mitPerson ? el('td', { text: z.person }) : null,
          el('td', {}, el('details', { class: 'herleitung' }, el('summary', { text: z.bezeichnung }),
            el('div', { class: 'text' }, el('p', {}, el('strong', { text: 'Herleitung: ' }), z.herleitung || '–'), el('p', {}, el('strong', { text: 'Rechtsgrundlage: ' }), z.rechtsgrundlage || '–')))),
          el('td', { class: 'betrag' }, betragOderWert(z)),
          el('td', {}, sicherheitsMarke(z.sicherheit))))))));
  }
  if (d.varianten.length) {
    wurzel.append(el('h3', { text: 'Varianten im Vergleich' }),
      el('p', { class: 'leise', text: 'Steuer = Einkommensteuer + Solidaritätszuschlag. Die Differenz bezieht sich auf die oben berechnete Fassung.' }),
      el('div', { class: 'tabelle-rahmen' }, el('table', {},
        el('thead', {}, el('tr', {}, el('th', { text: 'Variante' }), el('th', { class: 'betrag', text: 'Steuer' }), el('th', { class: 'betrag', text: 'Differenz' }), el('th', { text: 'Beschreibung' }))),
        el('tbody', {}, d.varianten.map((v) => el('tr', {},
          el('td', {}, el('strong', { text: v.titel })),
          el('td', { class: 'betrag', text: euro(v.steuer_cent) }),
          el('td', { class: 'betrag', text: (v.differenz_cent > 0 ? '+' : '') + euro(v.differenz_cent) }),
          el('td', { text: v.beschreibung })))))));
  }
  return wurzel;
};

// ---------- Vorjahre ----------

const vorjahrWahl = { jahr: null };

seiten.vorjahre = async () => {
  const vj = vorjahrWahl.jahr || (state.jahr - 1);
  const d = await api('GET', '/api/vorjahre?jahr=' + state.jahr + '&vorjahr=' + vj);
  const selJahr = el('input', { type: 'text', value: vj, inputmode: 'numeric', class: 'schmal', 'aria-label': 'Vorjahr' });
  selJahr.addEventListener('change', () => { const n = parseInt(selJahr.value, 10); if (n >= 2000 && n <= 2100) { vorjahrWahl.jahr = n; zeige(); } });

  const wurzel = el('div', {}, el('h2', { text: 'Vorjahre' }),
    el('p', {}, 'Werte aus dem Steuerbescheid des Jahres ', selJahr, ' im Vergleich mit der aktuellen Berechnung für ' + d.jahr + '. Abweichungen über 25 % sind markiert – sie sind kein Fehler, aber einen Blick wert.'));

  // Vorschläge aus Bescheiden
  if (d.vorschlaege.length) {
    wurzel.append(el('h3', { text: 'Erkannte Steuerbescheide' }));
    for (const v of d.vorschlaege) {
      const box = el('div', { class: 'box' }, el('p', {}, el('strong', { text: v.datei }), ' (Steuerjahr ' + v.steuerjahr + (v.aussteller ? ', ' + v.aussteller : '') + ') ', el('a', { href: '#/belege/' + v.beleg_id, text: 'Beleg ansehen' })));
      if (v.fehler) { box.append(el('p', { class: 'leise', text: v.fehler })); wurzel.append(box); continue; }
      if (!v.werte.length) { box.append(el('p', { class: 'leise', text: 'In diesem Bescheid wurden keine Werte erkannt. Du kannst sie unten von Hand eintragen.' })); wurzel.append(box); continue; }
      const alleGespeichert = v.werte.every((w) => w.gespeichert_cent === w.betrag_cent);
      box.append(el('div', { class: 'tabelle-rahmen' }, el('table', {},
        el('thead', {}, el('tr', {}, el('th', { text: 'Wert' }), el('th', { class: 'betrag', text: 'Erkannt' }), el('th', { class: 'betrag', text: 'Bereits gespeichert' }))),
        el('tbody', {}, v.werte.map((w) => el('tr', {}, el('td', { text: w.titel }), el('td', { class: 'betrag', text: euro(w.betrag_cent) }),
          el('td', { class: 'betrag', text: w.gespeichert_cent === null ? '–' : euro(w.gespeichert_cent) })))))));
      box.append(el('div', { class: 'leiste' }, el('button', {
        class: 'haupt', disabled: alleGespeichert, text: alleGespeichert ? 'Bereits übernommen' : 'Erkannte Werte als Vorjahreswerte bestätigen',
        onclick: async () => {
          const r = await tu(() => api('POST', '/api/vorjahre', { werte: v.werte.map((w) => ({ steuerjahr: w.steuerjahr, schluessel: w.schluessel, betrag_cent: w.betrag_cent })) }));
          if (r) { meldung('Vorjahreswerte gespeichert.'); await neuLaden(); }
        },
      }), el('span', { class: 'leise', text: 'Bitte vergleiche die Werte vorher mit dem Bescheid.' })));
      wurzel.append(box);
    }
  }

  // gespeicherte Werte + Vergleich
  wurzel.append(el('h3', { text: 'Werte des Jahres ' + d.vorjahr + ' im Vergleich' }));
  if (d.berechnungshinweis) wurzel.append(el('p', { class: 'hinweis pruefen', text: 'Ein Vergleich ist erst möglich, wenn die aktuelle Berechnung läuft: ' + d.berechnungshinweis }));
  if (!d.werte.length) {
    wurzel.append(el('p', { class: 'box', text: 'Für ' + d.vorjahr + ' sind noch keine Werte gespeichert. Lege den Steuerbescheid in den Eingang oder trage unten Werte ein.' }));
  } else {
    wurzel.append(el('div', { class: 'tabelle-rahmen' }, el('table', {},
      el('thead', {}, el('tr', {}, el('th', { text: 'Wert' }), el('th', { class: 'betrag', text: String(d.vorjahr) }), el('th', { class: 'betrag', text: String(d.jahr) + ' (aktuell)' }), el('th', { text: 'Abweichung' }), el('th', { text: '' }))),
      el('tbody', {}, d.werte.map((w) => el('tr', { class: w.auffaellig ? 'auffaellig' : '' },
        el('td', { text: w.titel }),
        el('td', { class: 'betrag', text: euro(w.betrag_cent) }),
        el('td', { class: 'betrag', text: w.hat_aktuell ? euro(w.aktuell_cent) : '–' }),
        el('td', {}, !w.hat_aktuell ? '–' : w.abweichung_prozent === null || w.abweichung_prozent === undefined
          ? (w.markierung ? marke('stark', '⚠ ' + w.markierung + ', Differenz ' + euro(w.differenz_cent)) : '0 %')
          : (w.auffaellig ? marke('stark', '⚠ ' + prozent(w.abweichung_prozent)) : prozent(w.abweichung_prozent))),
        el('td', {}, el('button', {
          class: 'gefahr', text: 'Löschen', onclick: async () => {
            if (!confirm('Vorjahreswert „' + w.titel + '“ löschen?')) return;
            const r = await tu(() => api('DELETE', '/api/vorjahre?jahr=' + d.vorjahr + '&schluessel=' + encodeURIComponent(w.schluessel)));
            if (r) await neuLaden();
          },
        }))))))));
  }

  // manuelle Eingabe
  const sel = el('select', { 'aria-label': 'Wert' }, state.meta.vorjahrsarten.map((k) => el('option', { value: k.schluessel, text: k.titel })));
  const betrag = el('input', { type: 'text', inputmode: 'decimal', placeholder: '0,00', 'aria-label': 'Betrag in Euro' });
  wurzel.append(el('div', { class: 'box' }, el('h3', { text: 'Vorjahreswert von Hand eintragen' }),
    el('div', { class: 'gitter' }, beschriftet('Wert', sel, 'breit'), beschriftet('Betrag (€, bei Nachzahlung negativ)', betrag)),
    el('div', { class: 'leiste' }, el('button', {
      class: 'haupt', text: 'Speichern', onclick: async () => {
        const c = parseBetrag(betrag.value);
        if (c === null) { meldung('Der Betrag ist keine gültige Zahl (z. B. 1.234,56).', true); return; }
        const r = await tu(() => api('POST', '/api/vorjahre', { werte: [{ steuerjahr: d.vorjahr, schluessel: sel.value, betrag_cent: c }] }));
        if (r) { meldung('Gespeichert.'); await neuLaden(); }
      },
    }))));
  return wurzel;
};

// ---------- Checkliste ----------

seiten.checkliste = async () => {
  const d = await api('GET', '/api/checkliste' + jahrQuery());
  const wurzel = el('div', {}, el('h2', { text: 'Was fehlt noch? – Steuerjahr ' + d.jahr }));
  if (!d.punkte.length) {
    wurzel.append(el('p', { class: 'box ok', text: '✓ Nichts offen: Alle Angaben sind ausgefüllt, alle Vorschläge bestätigt.' }));
    return wurzel;
  }
  const ziele = { belege: 'Zu den Belegen', positionen: 'Zu den Positionen', angaben: 'Zu den Angaben', erklaerung: 'Zur Erklärung' };
  for (const p of d.punkte) {
    wurzel.append(el('div', { class: 'box checkpunkt ' + p.stufe },
      el('h3', {}, marke(p.stufe === 'wichtig' ? 'unsicher' : 'stark', p.stufe === 'wichtig' ? 'wichtig' : 'Hinweis'), ' ', p.titel),
      el('p', { text: p.text }),
      p.details.length ? (p.details.length > 6
        ? el('details', {}, el('summary', { text: p.details.length + ' Einträge anzeigen' }), el('ul', { class: 'punkte' }, p.details.map((x) => el('li', { text: x }))))
        : el('ul', { class: 'punkte' }, p.details.map((x) => el('li', { text: x })))) : null,
      el('a', { class: 'knopf', href: '#/' + p.seite, text: ziele[p.seite] || 'Öffnen' })));
  }
  return wurzel;
};

// ---------- Start ----------

function jahrWaehler() {
  const sel = document.getElementById('jahr');
  for (let j = 2024; j <= 2027; j++) sel.append(el('option', { value: j, text: j }));
  try { const gemerkt = parseInt(localStorage.getItem('steuer.jahr'), 10); if (gemerkt >= 2024 && gemerkt <= 2027) state.jahr = gemerkt; } catch (e) { /* ohne Speicher */ }
  sel.value = state.jahr;
  sel.addEventListener('change', () => {
    state.jahr = parseInt(sel.value, 10);
    vorjahrWahl.jahr = null;
    try { localStorage.setItem('steuer.jahr', String(state.jahr)); } catch (e) { /* ohne Speicher */ }
    zeige();
  });
}

window.addEventListener('beforeprint', () => { for (const d of document.querySelectorAll('main details')) { d.dataset.warOffen = d.open ? '1' : '0'; d.open = true; } });
window.addEventListener('afterprint', () => { for (const d of document.querySelectorAll('main details')) { d.open = d.dataset.warOffen === '1'; } });
window.addEventListener('hashchange', zeige);

(async () => {
  try {
    state.meta = await api('GET', '/api/meta');
    state.jahr = state.meta.standardjahr;
  } catch (e) {
    document.getElementById('inhalt').textContent = e.message;
    return;
  }
  jahrWaehler();
  zeige();
})();
