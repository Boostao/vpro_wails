PRAGMA foreign_keys = ON;

CREATE TABLE datatypes (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR NOT NULL
);

CREATE TABLE units (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    desciption VARCHAR
);

CREATE TABLE fielddomains (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50)
);

CREATE TABLE fieldgroups (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    "order" INTEGER,
    metadata VARCHAR
);

CREATE TABLE lookuptables (
    id INTEGER NOT NULL PRIMARY KEY,
    filename VARCHAR(50) NOT NULL,
    description VARCHAR
);

CREATE TABLE lookupcaptions (
    id INTEGER NOT NULL PRIMARY KEY,
    lookuptableid INTEGER NOT NULL,
    columnno INTEGER NOT NULL,
    caption VARCHAR(50) NOT NULL,
    CONSTRAINT lookupcaptions_fkey FOREIGN KEY (lookuptableid) REFERENCES lookuptables(id)
);

CREATE TABLE lookupdata (
    id INTEGER NOT NULL PRIMARY KEY,
    lookuptableid INTEGER NOT NULL,
    code VARCHAR(100) NOT NULL,
    uri VARCHAR,
    description1 VARCHAR(255) NOT NULL,
    description2 VARCHAR(255) NOT NULL,
    description3 VARCHAR(255) NOT NULL,
    description4 VARCHAR(255) NOT NULL,
    CONSTRAINT lookuptablesdata_fkey01 FOREIGN KEY (lookuptableid) REFERENCES lookuptables(id)
);

CREATE TABLE covercodes (
    id INTEGER NOT NULL PRIMARY KEY,
    lookupdataid INTEGER NOT NULL,
    code VARCHAR NOT NULL,
    uri VARCHAR,
    percentage_mean REAL NOT NULL,
    percentage_min REAL,
    percentage_max REAL,
    CONSTRAINT covercodes_fkey01 FOREIGN KEY (lookupdataid) REFERENCES lookupdata(id)
);

CREATE TABLE custodians (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR NOT NULL,
    institution VARCHAR,
    email VARCHAR,
    metadata VARCHAR
);

CREATE TABLE datasetgroups (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    metadata VARCHAR
);

CREATE TABLE profiles (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL
);

CREATE TABLE taxonlists (
    id INTEGER NOT NULL PRIMARY KEY,
    sbstaxonlistid INTEGER,
    name VARCHAR(50) NOT NULL,
    metadata VARCHAR
);

CREATE TABLE taxongroups (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    metadata VARCHAR
);

CREATE TABLE taxonranks (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    metadata VARCHAR
);

CREATE TABLE vegetationlayers (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    abbreviation VARCHAR(50),
    description VARCHAR
);

CREATE TABLE metadata (
    "key" VARCHAR NOT NULL PRIMARY KEY,
    "value" VARCHAR NOT NULL
);

CREATE TABLE projects (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    metadata VARCHAR
);

CREATE TABLE datasets (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    datasetgroupid INTEGER NOT NULL,
    profileid INTEGER NOT NULL,
    taxonlistid INTEGER NOT NULL,
    custodianid INTEGER,
    deputycustodianid INTEGER,
    givdid VARCHAR,
    guid VARCHAR,
    metadata VARCHAR,
    CONSTRAINT datasets_fkey01 FOREIGN KEY (datasetgroupid) REFERENCES datasetgroups(id),
    CONSTRAINT datasets_fkey02 FOREIGN KEY (taxonlistid) REFERENCES taxonlists(id),
    CONSTRAINT datasets_fkey03 FOREIGN KEY (profileid) REFERENCES profiles(id),
    CONSTRAINT datasets_fkey04 FOREIGN KEY (custodianid) REFERENCES custodians(id),
    CONSTRAINT datasets_fkey05 FOREIGN KEY (deputycustodianid) REFERENCES custodians(id)
);

CREATE TABLE projectdatasets (
    projectid INTEGER NOT NULL,
    datasetid INTEGER NOT NULL,
    CONSTRAINT projectdatasets_fkey01 FOREIGN KEY (projectid) REFERENCES projects(id),
    CONSTRAINT projectdatasets_fkey02 FOREIGN KEY (datasetid) REFERENCES datasets(id)
);

CREATE TABLE fields (
    id INTEGER NOT NULL PRIMARY KEY,
    fielddomainid INTEGER NOT NULL,
    fieldgroupid INTEGER,
    lookuptableid INTEGER,
    unitid INTEGER,
    fieldname VARCHAR(20) NOT NULL,
    fieldtype VARCHAR(1) NOT NULL,
    description VARCHAR,
    datatypeid INTEGER NOT NULL,
    width SMALLINT NOT NULL,
    decimals SMALLINT NOT NULL,
    mandatory BOOLEAN,
    readonly BOOLEAN,
    valmin REAL,
    valmax REAL,
    valin VARCHAR,
    hint VARCHAR,
    CONSTRAINT fields_fkey01 FOREIGN KEY (unitid) REFERENCES units(id),
    CONSTRAINT fields_fkey02 FOREIGN KEY (lookuptableid) REFERENCES lookuptables(id),
    CONSTRAINT fields_fkey03 FOREIGN KEY (fieldgroupid) REFERENCES fieldgroups(id),
    CONSTRAINT fields_fkey04 FOREIGN KEY (datatypeid) REFERENCES datatypes(id),
    CONSTRAINT fields_fkey05 FOREIGN KEY (fielddomainid) REFERENCES fielddomains(id)
);

CREATE TABLE profiledata (
    id INTEGER NOT NULL PRIMARY KEY,
    profileid INTEGER NOT NULL,
    fieldid INTEGER NOT NULL,
    "order" INTEGER,
    "show" BOOLEAN,
    mandatory BOOLEAN,
    readonly BOOLEAN,
    CONSTRAINT profiledata_fkey01 FOREIGN KEY (profileid) REFERENCES profiles(id),
    CONSTRAINT profiledata_fkey02 FOREIGN KEY (fieldid) REFERENCES fields(id)
);

CREATE TABLE locations (
    id INTEGER NOT NULL PRIMARY KEY,
    the_geom GEOMETRY NOT NULL
);

CREATE TABLE plots (
    id INTEGER NOT NULL PRIMARY KEY,
    datasetid INTEGER NOT NULL,
    locationid INTEGER,
    locationuncertainty INTEGER,
    metadata VARCHAR,
    CONSTRAINT plots_fkey01 FOREIGN KEY (datasetid) REFERENCES datasets(id),
    CONSTRAINT plots_fkey02 FOREIGN KEY (locationid) REFERENCES locations(id)
);

CREATE TABLE plotobservations (
    id INTEGER NOT NULL PRIMARY KEY,
    plotid INTEGER NOT NULL,
    guid VARCHAR NOT NULL,
    changedby VARCHAR(50),
    changedwhen DOUBLE,
    quality SMALLINT,
    deleted BOOLEAN,
    synoptic BOOLEAN,
    nrofplotobservations INTEGER,
    accessregime SMALLINT,
    CONSTRAINT plotobservations_fkey01 FOREIGN KEY (plotid) REFERENCES plots(id)
);

CREATE TABLE plotobservationheaderdata (
    id INTEGER NOT NULL PRIMARY KEY,
    plotobservationid INTEGER NOT NULL,
    fieldid INTEGER NOT NULL,
    lookupdataid INTEGER,
    valuestring VARCHAR,
    valueinteger INTEGER,
    valuefloat DOUBLE,
    deleted BOOLEAN,
    CONSTRAINT plotobservationheaderdata_fkey01 FOREIGN KEY (plotobservationid) REFERENCES plotobservations(id),
    CONSTRAINT plotobservationheaderdata_fkey02 FOREIGN KEY (fieldid) REFERENCES fields(id),
    CONSTRAINT plotobservationheaderdata_fkey03 FOREIGN KEY (lookupdataid) REFERENCES lookupdata(id)
);

CREATE TABLE pictures (
    id INTEGER NOT NULL PRIMARY KEY,
    plotobservationid INTEGER NOT NULL,
    url VARCHAR,
    author VARCHAR,
    date DOUBLE,
    longitude DOUBLE,
    latitude DOUBLE,
    description VARCHAR,
    CONSTRAINT pictures_fkey01 FOREIGN KEY (plotobservationid) REFERENCES plotobservations(id)
);

CREATE TABLE associa (
    id INTEGER NOT NULL PRIMARY KEY,
    plotobservationid INTEGER NOT NULL,
    lookupdataid INTEGER NOT NULL,
    combinedindex REAL NOT NULL,
    normalised_likelyhood REAL NOT NULL,
    incompleteness REAL NOT NULL,
    weirdness REAL NOT NULL,
    CONSTRAINT associa_fkey01 FOREIGN KEY (plotobservationid) REFERENCES plotobservations(id),
    CONSTRAINT associa_fkey02 FOREIGN KEY (lookupdataid) REFERENCES lookupdata(id)
);

CREATE TABLE taxa (
    id INTEGER NOT NULL PRIMARY KEY,
    taxonlistid INTEGER NOT NULL,
    taxongroupid INTEGER,
    taxonrankid INTEGER,
    parent INTEGER,
    scientificname VARCHAR NOT NULL,
    vernacularname VARCHAR,
    author VARCHAR,
    tv2speciesid INTEGER,
    tv2validid INTEGER,
    sbsspeciesid INTEGER,
    lettercode VARCHAR(7),
    issynonym INTEGER,
    uri VARCHAR,
    metadata VARCHAR,
    CONSTRAINT species_fkey01 FOREIGN KEY (taxonlistid) REFERENCES taxonlists(id),
    CONSTRAINT species_fkey02 FOREIGN KEY (taxongroupid) REFERENCES taxongroups(id),
    CONSTRAINT species_fkey03 FOREIGN KEY (taxonrankid) REFERENCES taxonranks(id)
);

CREATE TABLE traitdata (
    id INTEGER NOT NULL PRIMARY KEY,
    taxonid INTEGER NOT NULL,
    fieldid INTEGER NOT NULL,
    value VARCHAR NOT NULL,
    CONSTRAINT traitdata_fkey01 FOREIGN KEY (taxonid) REFERENCES taxa(id),
    CONSTRAINT traitdata_fkey02 FOREIGN KEY (fieldid) REFERENCES fields(id)
);

CREATE TABLE speciesobservations (
    id INTEGER NOT NULL PRIMARY KEY,
    plotobservationid INTEGER NOT NULL,
    taxonid INTEGER NOT NULL,
    vegetationlayerid INTEGER NOT NULL,
    covercodeid INTEGER NOT NULL,
    originaltaxonconcept VARCHAR,
    metadata VARCHAR,
    deleted BOOLEAN,
    CONSTRAINT speciesdata_fkey01 FOREIGN KEY (plotobservationid) REFERENCES plotobservations(id),
    CONSTRAINT speciesdata_fkey02 FOREIGN KEY (taxonid) REFERENCES taxa(id),
    CONSTRAINT speciesdata_fkey03 FOREIGN KEY (vegetationlayerid) REFERENCES vegetationlayers(id),
    CONSTRAINT speciesdata_fkey04 FOREIGN KEY (covercodeid) REFERENCES covercodes(id)
);

CREATE TABLE speciesobservationdata (
    id INTEGER NOT NULL PRIMARY KEY,
    speciesobservationid INTEGER NOT NULL,
    fieldid INTEGER NOT NULL,
    lookupdataid INTEGER,
    valuestring VARCHAR,
    valueinteger INTEGER,
    valuefloat REAL,
    deleted BOOLEAN,
    CONSTRAINT observationsspeciesdata_fkey01 FOREIGN KEY (speciesobservationid) REFERENCES speciesobservations(id),
    CONSTRAINT observationsspeciesdata_fkey02 FOREIGN KEY (fieldid) REFERENCES fields(id)
);

CREATE INDEX associa_index01 ON associa (plotobservationid);
CREATE UNIQUE INDEX associa_index02 ON associa (plotobservationid, lookupdataid);
CREATE UNIQUE INDEX covercodes_index01 ON covercodes (lookupdataid, code);
CREATE UNIQUE INDEX custodians_index01 ON custodians (name);
CREATE UNIQUE INDEX datasetgroups_index01 ON datasetgroups (name);
CREATE INDEX datasets_index01 ON datasets (datasetgroupid);
CREATE UNIQUE INDEX datasets_index02 ON datasets (name);
CREATE UNIQUE INDEX datasetgroups_index02 ON fieldgroups (name);
CREATE UNIQUE INDEX fields_index01 ON fields (fielddomainid, fieldname, fieldtype);
CREATE INDEX locations_index01 ON locations (the_geom);
CREATE UNIQUE INDEX lookupcaptions_index01 ON lookupcaptions (lookuptableid, columnno);
CREATE UNIQUE INDEX lookuptabledata_index01 ON lookupdata (lookuptableid, code, description1, description2, description3, description4);
CREATE UNIQUE INDEX lookuptables_index01 ON lookuptables (filename, description);
CREATE INDEX pictures_index01 ON pictures (plotobservationid);
CREATE INDEX plotobservationheaderdata_index01 ON plotobservationheaderdata (plotobservationid);
CREATE INDEX plotobservationheaderdata_index02 ON plotobservationheaderdata (fieldid);
CREATE INDEX plotobservations_index01 ON plotobservations (plotid);
CREATE UNIQUE INDEX plotobservations_index02 ON plotobservations (guid);
CREATE INDEX plots_index01 ON plots (datasetid);
CREATE INDEX profiledata_index01 ON profiledata (profileid, fieldid);
CREATE UNIQUE INDEX profiles_index01 ON profiles (name);
CREATE UNIQUE INDEX projects_index01 ON projects (name);
CREATE INDEX speciesobservationdata_index01 ON speciesobservationdata (speciesobservationid);
CREATE INDEX speciesobservationdata_index02 ON speciesobservationdata (fieldid);
CREATE INDEX observationsspecies_index01 ON speciesobservations (plotobservationid);
CREATE UNIQUE INDEX observationsspecies_index02 ON speciesobservations (plotobservationid, taxonid, vegetationlayerid, originaltaxonconcept);
CREATE INDEX observationsspecies_index03 ON speciesobservations (taxonid);
CREATE UNIQUE INDEX taxa_index01 ON taxa (taxonlistid, scientificname, tv2speciesid);
CREATE INDEX taxa_index02 ON taxa (tv2speciesid);
CREATE INDEX taxa_index03 ON taxa (taxonlistid);
CREATE INDEX taxa_index04 ON taxa (sbsspeciesid);
CREATE UNIQUE INDEX speciesgroups_index01 ON taxongroups (name);
CREATE UNIQUE INDEX floras_index01 ON taxonlists (name);
CREATE INDEX traitdata_index01 ON traitdata (taxonid);
CREATE UNIQUE INDEX traitdata_index02 ON traitdata (taxonid, fieldid);
CREATE UNIQUE INDEX units_index01 ON units (name);

INSERT INTO datatypes (id, name) VALUES (1, 'TEXT'), (2, 'INTEGER'), (3, 'REAL'), (4, 'DATE'), (5, 'BOOLEAN');
INSERT INTO units (id, name, desciption) VALUES (1, '%', 'Percentage'), (2, 'm', 'Meters'), (3, 'deg', 'Degrees'), (4, 'm2', 'Square meters');
INSERT INTO fielddomains (id, name) VALUES (1, 'Administrative and header data'), (2, 'Species attributes'), (3, 'Species trait data');
INSERT INTO fieldgroups (id, name, "order") VALUES (1, 'General', 1), (2, 'Topography', 2), (3, 'Classification', 3);

INSERT INTO vegetationlayers (id, name, abbreviation, description) VALUES
    (0, '-', '-', 'no layer defined'),
    (1, 'Tree layer 1', 't1', 'Tree layer 1'),
    (2, 'Tree layer 2', 't2', 'Tree layer 2'),
    (3, 'Tree layer 3', 't3', 'Tree layer 3'),
    (4, 'Shrub layer 1', 's1', 'Shrub layer 1'),
    (5, 'Shrub layer 2', 's2', 'Shrub layer 2'),
    (6, 'Herb layer', 'hl', 'Herb layer'),
    (7, 'Juvenile layer', 'jl', 'Juvenile layer'),
    (8, 'Seedling layer', 'sl', 'Seedling layer'),
    (9, 'Cryptogam layer', 'ml', 'Cryptogam layer'),
    (10, 'Epiphytes', 'ep', 'Epiphytes');

INSERT INTO lookuptables (id, filename, description) VALUES
    (1, 'tvscale', 'Cover abundance scales'),
    (2, 'tvauthor', 'Authors');
INSERT INTO lookupcaptions (id, lookuptableid, columnno, caption) VALUES
    (1, 1, 1, 'Code'), (2, 1, 2, 'Description'),
    (3, 2, 1, 'Code'), (4, 2, 2, 'Author name');
INSERT INTO lookupdata (id, lookuptableid, code, description1, description2, description3, description4) VALUES
    (1, 1, '01', 'Braun-Blanquet', '', '', ''),
    (2, 2, 'HENK', 'S.M. Hennekens', '', '', '');

INSERT INTO covercodes (id, lookupdataid, code, percentage_mean, percentage_min, percentage_max) VALUES
    (1, 1, 'r', 0.1, 0.01, 0.5),
    (2, 1, '+', 0.5, 0.1, 1.0),
    (3, 1, '1', 3.0, 1.0, 5.0),
    (4, 1, '2a', 8.75, 5.0, 12.5),
    (5, 1, '2b', 18.75, 12.5, 25.0),
    (6, 1, '3', 37.5, 25.0, 50.0),
    (7, 1, '4', 62.5, 50.0, 75.0);

INSERT INTO custodians (id, name, institution) VALUES (1, 'Alterra WENR', 'Wageningen University & Research');
INSERT INTO datasetgroups (id, name) VALUES (1, 'External');
INSERT INTO profiles (id, name) VALUES (1, 'Standard');
INSERT INTO taxonlists (id, name) VALUES (1, 'ExternalFlora');
INSERT INTO taxongroups (id, name) VALUES (1, 'Vascular plants'), (3, 'Mosses'), (4, 'Lichens');
INSERT INTO taxonranks (id, name) VALUES (2, 'Species'), (4, 'Variety');
INSERT INTO projects (id, name) VALUES (1, 'ExtTV3Proj');
INSERT INTO datasets (id, name, datasetgroupid, profileid, taxonlistid, custodianid, guid) VALUES
    (1, 'ExtTV3Dataset', 1, 1, 1, 1, '11111111-2222-4333-8444-555555555555');
INSERT INTO projectdatasets (projectid, datasetid) VALUES (1, 1);
INSERT INTO metadata ("key", "value") VALUES ('dbversion', '3.9');

-- Deliberately use non-sequential field IDs to prove reader matches by fieldname, not hardcoded ID
INSERT INTO fields (id, fielddomainid, fieldgroupid, lookuptableid, unitid, fieldname, fieldtype, datatypeid, width, decimals) VALUES
    (10, 1, 1, NULL, NULL, 'RELEVE_NR', 'N', 2, 6, 0),
    (20, 1, 1, NULL, NULL, 'DATE', 'D', 4, 10, 0),
    (30, 1, 2, NULL, 4, 'SURF_AREA', 'N', 3, 8, 2),
    (40, 1, 2, NULL, 2, 'ALTITUDE', 'N', 2, 5, 0),
    (50, 1, 2, NULL, 3, 'EXPOSITION', 'N', 2, 3, 0),
    (60, 1, 2, NULL, 3, 'INCLINATIO', 'N', 3, 5, 1),
    (70, 1, 1, 2, NULL, 'AUTHOR', 'C', 1, 4, 0),
    (80, 1, 3, NULL, NULL, 'ZONE', 'C', 1, 4, 0),
    (90, 1, 3, NULL, NULL, 'SUBZONE', 'C', 1, 8, 0),
    (95, 1, 3, NULL, NULL, 'SITE_UNIT', 'C', 1, 50, 0),
    (98, 1, 1, NULL, NULL, 'REMARKS', 'C', 1, 255, 0),
    (201, 2, NULL, NULL, 2, 'HEIGHT', 'N', 3, 6, 2);

INSERT INTO profiledata (id, profileid, fieldid, "order", "show") VALUES
    (1, 1, 10, 1, 1), (2, 1, 20, 2, 1), (3, 1, 30, 3, 1), (4, 1, 40, 4, 1);

-- Plot 1: located via WKT POINT(-127.125 54.75) (NO header LATITUDE/LONGITUDE rows!), Delphi TDateTime 45492.0 = 2024-07-19
-- Plot 2: deleted plotobservation (deleted = 1) -> must be ignored on import!
INSERT INTO locations (id, the_geom) VALUES (1, 'POINT(-127.125 54.75)');
INSERT INTO plots (id, datasetid, locationid, locationuncertainty) VALUES (1, 1, 1, 15), (2, 1, NULL, NULL);
INSERT INTO plotobservations (id, plotid, guid, changedby, changedwhen, quality, deleted, synoptic, accessregime) VALUES
    (1, 1, 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee', 'hennekens', 45492.5, 3, NULL, 0, 0),
    (2, 2, 'ffffffff-bbbb-4ccc-8ddd-eeeeeeeeeeee', 'hennekens', 45492.5, 3, 1, 0, 0);

INSERT INTO plotobservationheaderdata (id, plotobservationid, fieldid, lookupdataid, valuestring, valueinteger, valuefloat, deleted) VALUES
    (1, 1, 10, NULL, NULL, 501, NULL, NULL),
    (2, 1, 20, NULL, NULL, NULL, 45492.0, NULL),
    (3, 1, 30, NULL, NULL, NULL, 400.0, NULL),
    (4, 1, 40, NULL, NULL, 910, NULL, NULL),
    (5, 1, 50, NULL, NULL, 180, NULL, NULL),
    (6, 1, 60, NULL, NULL, NULL, 24.5, NULL),
    (7, 1, 70, 2, NULL, NULL, NULL, NULL),
    (8, 1, 80, NULL, 'ICH', NULL, NULL, NULL),
    (9, 1, 90, NULL, 'mc2', NULL, NULL, NULL),
    (10, 1, 95, NULL, 'ICHmc2/01', NULL, NULL, NULL),
    (11, 1, 98, NULL, 'External TV3 WKT + Delphi TDateTime test', NULL, NULL, NULL),
    (12, 2, 10, NULL, NULL, 999, NULL, NULL);

-- Taxa matching VLists.USysAllSpecs by scientific name (plus synonym Chamaecyparis nootkatensis -> CALLNOO and 8-char PINUCON1)
INSERT INTO taxa (id, taxonlistid, taxongroupid, taxonrankid, scientificname, vernacularname, author, tv2speciesid, lettercode, issynonym) VALUES
    (1, 1, 1, 2, 'Pseudotsuga menziesii', 'Douglas-fir', '(Mirbel) Franco', 101, 'PSEDMEN', 0),
    (2, 1, 1, 2, 'Vaccinium membranaceum', 'black huckleberry', 'Dougl. ex Hook.', 102, 'VACCMEM', 0),
    (3, 1, 1, 2, 'Cornus canadensis', 'bunchberry', 'L.', 103, 'CORNCAN', 0),
    (4, 1, 3, 2, 'Hylocomium splendens', 'step moss', '(Hedw.) Schimp.', 104, 'HYLOSPL', 0),
    (5, 1, 4, 2, 'Alectoria sarmentosa', 'witchs-hair', '(Ach.) Ach.', 105, 'ALECSAR', 0),
    (6, 1, 1, 4, 'Pinus contorta var. contorta', 'shore pine', 'Dougl. ex Loud.', 106, 'PINUCON', 0),
    (7, 1, 1, 2, 'Chamaecyparis nootkatensis', 'yellow-cedar', '(D. Don) Spach', 107, 'CHAMNOO', 1);

-- Species observations across layers:
-- 1 (t1): Pseudotsuga menziesii (37.5% = code '3', height=31.5m) + Chamaecyparis nootkatensis (8.75% = '2a') + Pinus contorta var. contorta (3.0% = '1')
-- 4 (s1): Vaccinium membranaceum (18.75% = '2b')
-- 6 (hl): Cornus canadensis (8.75% = '2a')
-- 7 (jl): Vaccinium membranaceum (woody -> Cover5! 3.0% = '1', height=0.8m) AND Cornus canadensis (herb -> Cover6! 0.5% = '+')
-- 8 (sl): Pseudotsuga menziesii (seedling -> Cover6! 0.5% = '+')
-- 9 (ml): Hylocomium splendens (cryptogam -> Cover7! 62.5% = '4')
-- 10 (ep): Alectoria sarmentosa (epiphyte -> Cover10! 0.1% = 'r')
-- Plus one deleted observation (deleted=1) that must be ignored!
INSERT INTO speciesobservations (id, plotobservationid, taxonid, vegetationlayerid, covercodeid, originaltaxonconcept, deleted) VALUES
    (1, 1, 1, 1, 6, 'Pseudotsuga menziesii', NULL),
    (2, 1, 7, 1, 4, 'Chamaecyparis nootkatensis', NULL),
    (3, 1, 6, 1, 3, 'Pinus contorta var. contorta', NULL),
    (4, 1, 2, 4, 5, 'Vaccinium membranaceum', NULL),
    (5, 1, 3, 6, 4, 'Cornus canadensis', NULL),
    (6, 1, 2, 7, 3, 'Vaccinium membranaceum', NULL),
    (7, 1, 3, 7, 2, 'Cornus canadensis', NULL),
    (8, 1, 1, 8, 2, 'Pseudotsuga menziesii', NULL),
    (9, 1, 4, 9, 7, 'Hylocomium splendens', NULL),
    (10, 1, 5, 10, 1, 'Alectoria sarmentosa', NULL),
    (11, 1, 5, 6, 6, 'Alectoria sarmentosa (deleted)', 1);

INSERT INTO speciesobservationdata (id, speciesobservationid, fieldid, valuefloat, deleted) VALUES
    (1, 1, 201, 31.5, NULL),
    (2, 6, 201, 0.8, NULL);
