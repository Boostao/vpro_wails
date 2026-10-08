const cell = (storage, value) => ({
  storage, text: storage === 'text' ? value : null, integer: storage === 'integer' ? value : null,
  real: storage === 'real' ? value : null, blobHex: storage === 'blob' ? value : null,
});
function review({ extended = false, extendedCoverOnly = false, applicationId = '1' } = {}) {
  const columns = [
    ['ID', 'PlotNumber', 'Species', 'Cover1', 'Cover2', 'Cover3', 'TotalA', 'HeightA', 'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB', 'HeightB', 'Collected'],
    ['ID', 'PlotNumber', 'Species', 'Cover6', 'Height6', 'Collected'],
    ['ID', 'PlotNumber', 'Species', 'Cover7', 'Cover8', 'Cover9', 'Collected'],
  ];
  return columns.map((names, i) => ({
    Form: [extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC', 'SubVegC-SIVI', 'SubVegD-SIVI'][i],
    Query: ['USysVegA', 'USysVegC', 'USysVegD'][i], Columns: names,
    Rows: [{ rowId: '9007199254740993', cells: names.map(name => name === 'ID' ? cell('integer', applicationId) :
      name === 'PlotNumber' ? cell('text', 'P') : name === 'Species' ? cell('text', 'RAW') :
      name === [extendedCoverOnly ? 'Cover5a' : 'Cover1', 'Cover6', 'Cover9'][i] ? cell('real', 0) :
      name === 'HeightA' ? cell('real', 2) : name === 'HeightB' ? cell('text', '') : cell('null')) }],
  }));
}
module.exports = { cell, review };
