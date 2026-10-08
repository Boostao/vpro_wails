import type { GoogleEarthKMLReview } from '../bindings/github.com/boostao/vpro-wails';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';

const namespace = 'http://earth.google.com/kml/2.1';
const header = '<?xml version="1.0" encoding="UTF-8"?>\n';

export function googleEarthXMLText(value: string): boolean {
  if (typeof value !== 'string' || !wellFormedUTF16(value)) return false;
  for (const character of value) {
    const code = character.codePointAt(0)!;
    if (code !== 9 && code !== 10 && code !== 13 &&
        !(code >= 0x20 && code <= 0xd7ff || code >= 0xe000 && code <= 0xfffd || code >= 0x10000 && code <= 0x10ffff)) return false;
  }
  return true;
}

async function inspectKML(xml: string, title: string, count: number, yieldTask: () => Promise<void>): Promise<boolean> {
  const document = new DOMParser().parseFromString(xml, 'application/xml');
  const root = document.documentElement;
  if (document.doctype || document.getElementsByTagName('parsererror').length || !root ||
      root.localName !== 'kml' || root.namespaceURI !== namespace || root.children.length !== 1) return false;
  const body = root.children[0];
  if (body.localName !== 'Document' || body.children.length !== count + 1 ||
      body.children[0].localName !== 'name' || body.children[0].textContent !== title) return false;
  for (const element of Array.from(document.getElementsByTagName('*'))) {
    if (element.namespaceURI !== namespace || (element === root
      ? element.attributes.length !== 1 || element.getAttribute('xmlns') !== namespace
      : element.attributes.length !== 0)) return false;
  }
  function scalar(element: Element, name: string): boolean {
    return element.localName === name && element.children.length === 0;
  }
  if (!scalar(body.children[0], 'name')) return false;
  const marks = Array.from(body.children).slice(1);
  for (let i = 0; i < marks.length; i++) {
    const mark = marks[i];
    if (mark.localName !== 'Placemark' || mark.children.length !== 3 ||
        !scalar(mark.children[0], 'name') || !scalar(mark.children[1], 'description') ||
        /[<>]/.test(mark.children[1].textContent ?? '') || mark.children[2].localName !== 'Point' ||
        mark.children[2].children.length !== 1 || !scalar(mark.children[2].children[0], 'coordinates')) return false;
    const coordinates = (mark.children[2].children[0].textContent ?? '').split(',');
    if (coordinates.length !== 3 || coordinates[2] !== '0' ||
        !coordinates.slice(0, 2).every(number => /^-?(?:0|[1-9]\d*)(?:\.\d+)?$/.test(number))) return false;
    const longitude = Number(coordinates[0]), latitude = Number(coordinates[1]);
    if (!Number.isFinite(longitude) || !Number.isFinite(latitude) ||
        longitude < -180 || longitude > 180 || latitude < -90 || latitude > 90) return false;
    if ((i + 1) % 100 === 0) await yieldTask();
  }
  return true;
}

export async function validateGoogleEarthKMLReview(value: GoogleEarthKMLReview | null, scope: GoogleEarthScope,
  descriptionField: string, title: string,
  inspect: (xml: string, title: string, count: number, yieldTask: () => Promise<void>) => boolean | Promise<boolean> = inspectKML,
  yieldTask: () => Promise<void> = () => new Promise(resolve => setTimeout(resolve, 0))): Promise<GoogleEarthKMLReview> {
  value = value ? structuredClone(value) : null;
  if (!value || !ownedScope(value, scope) || !googleEarthXMLText(title) ||
      !googleEarthXMLText(descriptionField) || !descriptionField ||
      value.descriptionField !== descriptionField || value.title !== title ||
      !Number.isSafeInteger(value.placemarkCount) || value.placemarkCount < 0 ||
      !Number.isSafeInteger(value.byteCount) || value.byteCount < 1 ||
      !googleEarthXMLText(value.kml) || new TextEncoder().encode(value.kml).length !== value.byteCount ||
      !value.kml.startsWith(header) || /<\?|<!/.test(value.kml.slice(header.length))) {
    throw new Error('KML preview has a foreign/incomplete scope, title, byte count or XML shape; no partial preview accepted.');
  }
  await yieldTask();
  if (!await inspect(value.kml, title, value.placemarkCount, yieldTask)) {
    throw new Error('KML preview has an incomplete XML shape; no partial preview accepted.');
  }
  return value;
}
