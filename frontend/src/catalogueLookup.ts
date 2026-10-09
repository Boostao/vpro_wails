import { QualityLookup, type PlotQualityChoice, type QualityView } from './qualityEditor';
import { ReadRequests } from './readRequests';

export class CatalogueLookup extends QualityLookup {
  constructor(fetch: (force: boolean, requests: ReadRequests) => Promise<PlotQualityChoice[]>,
    state: (view: QualityView) => void, label: string) {
    const requests = new ReadRequests();
    super(force => fetch(force, requests), state, label, () => requests.cancelAll());
  }
}
