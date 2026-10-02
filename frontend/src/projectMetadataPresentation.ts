import definition from '../../resources/project-metadata-layout.json';

const source = definition.forms.find(form => form.name === 'frmProjectMetaData');
if (!source || source.recordSource !== 'UsysMetadata') throw new Error('Metadata source layout/record binding is unavailable.');

const labels: Record<string, string> = {
  StartDate: 'Start year', EndDate: 'End year', BAPID: 'BAP ID',
  ExtraVegFieldDescription: 'Extra vegetation field description', DataCustodian: 'Data custodian',
  StorageLocation: 'Storage location', Notes: 'Notes', DateLastEdited: 'Last edited',
  CollectedCompleteOther: 'Other data collected completely', CollectedPartialOther: 'Other data collected partially',
  CollectedNoneOther: 'Other data not collected',
};
const categories: Record<string, string> = {
  Site: 'Site', Veg: 'Vegetation', Soil: 'Soil', Terrain: 'Terrain', Mens: 'Mensuration',
  CWD: 'Coarse woody debris', WildTree: 'Wildlife trees', SoilChem: 'Soil chemistry',
  WildlifeHabitatAssessment: 'Wildlife habitat assessment',
};
export function metadataLabel(name: string): string {
  if (labels[name]) return labels[name];
  for (const prefix of ['Collected', 'DataQuality']) {
    if (name.startsWith(prefix) && categories[name.slice(prefix.length)]) {
      return `${categories[name.slice(prefix.length)]} — ${prefix === 'Collected' ? 'collection' : 'data quality'}`;
    }
  }
  if (name.endsWith('Other')) return `${metadataLabel(name.slice(0, -5))} — other`;
  const found = source!.fields.filter(field => field.column?.toLowerCase() === name.toLowerCase());
  if (found.length !== 1 || !found[0].caption) throw new Error(`Metadata ${name} has no unambiguous source/associated label.`);
  return name.startsWith('Cover') ? `${found[0].caption} layer description` : found[0].caption;
}
export const metadataGroups = [
  { title: 'Project and field team', names: ['ProjectTitle', 'ProjectType', 'ProjectTypeOther', 'EcosysCollectionStandard',
    'EcosysCollectionStandardOther', 'CoordinatingAgency', 'ProponentFunder', 'FieldCompanyAgency', 'FieldLeader',
    'FieldDataCollectionTeam', 'ProjectPurpose', 'GeographicStudyArea', 'GeographicStudyRegion', 'StartDate', 'EndDate',
    'NumberOfFS882Plots', 'NumberOfSiteVisits', 'BAPID'] },
  { title: 'Field methods', names: ['VegCoverMethod', 'VegCoverMethodOther', 'PlotMethod', 'PlotMethodOther',
    'MensurationMethod', 'MensurationMethodOther', 'ExtraVegFieldDescription'] },
  { title: 'Collection and data quality', names: Object.keys(categories).flatMap(name => [`Collected${name}`, `DataQuality${name}`])
    .concat(['CollectedCompleteOther', 'CollectedPartialOther', 'CollectedNoneOther']) },
  { title: 'Georeferencing', names: ['GeoRefMethod', 'GeoRefMethodOther', 'Datum', 'DatumOther', 'CoordinateSystem', 'CoordinateSystemOther'] },
  { title: 'Vegetation layer descriptions', names: ['CoverADescription', 'CoverA1Description', 'CoverA2Description',
    'CoverA3Description', 'CoverBDescription', 'CoverB1Description', 'CoverB2Description', 'CoverB2aDescription',
    'CoverB2bDescription', 'CoverB2cDescription', 'CoverCDescription', 'CoverDDescription', 'Cover8Description',
    'Cover9Description', 'Cover10Description'] },
  { title: 'Custody and notes', names: ['DataCustodian', 'StorageLocation', 'Notes'] },
];
