package listcatalog

type ListDefinition struct {
	Name string
	Rows int
}

type Profile struct {
	Exporter       string
	SourceSHA256   string
	SnapshotSHA256 string
	SQL            string
	Lists          []ListDefinition
}

func SiteProfile() Profile {
	return Profile{Exporter, SourceSHA256, SnapshotSHA256, SourceSQL,
		[]ListDefinition{{"Exposure", 12}, {"SiteDisturbance", 127}}}
}

const RegionSnapshotSHA256 = "73661609828f78625b083327a750e10819dc05a1a20e78e8c9c91b177ae739ee"

func RegionProfile() Profile {
	return Profile{"native-access-dao-region-codes-v1", SourceSHA256, RegionSnapshotSHA256,
		`Region: SELECT * FROM USysTableOfLists WHERE ListName="Region" ORDER BY ItemOrder; Ecosection: SELECT * FROM USysTableOfLists WHERE ListName="ecosection"`,
		[]ListDefinition{{"Ecosection", 137}, {"Region", 27}}}
}

const SoilSnapshotSHA256 = "20e309d5e34445109d2024406e451e601257b2d94d877c54dd9a87e8b79d0f5d"
const SoilNativeTypedCellsSHA256 = "db6ff589a8a393877b7a3fcc82bd1886b3603181d5dd0a7c64920c4a8a04558f"

func SoilSourceSQL(list string) string {
	return DAOSourceSQL(list)
}

func DAOSourceSQL(list string) string {
	return `SELECT [ListName],[ListFilter],[ItemOrder],[Item],[ItemDescription],[FieldUsedIn],[ValidateLoops],[Validate],[Note],[Flag] FROM [USysTableOfLists] WHERE [ListName]='` + list + `' ORDER BY [ItemOrder];`
}

const GeologySnapshotSHA256 = "2e8585380babdb7110615a43321900edeac882d573f25c99ed1780f93e0956f6"
const GeologyNativeTypedCellsSHA256 = "926d81a2100d88ac6ba6b9f548ae9ac7d3b48b75a41016a9eba1836d4a9241d0"

func GeologyProfile() Profile {
	return Profile{"current-readonly-dao-bedrock-codes-v1", SourceSHA256, GeologySnapshotSHA256,
		DAOSourceSQL("BedrockType"), []ListDefinition{{"BedrockType", 87}}}
}

func SoilProfile() Profile {
	return Profile{"current-readonly-dao-soil-codes-v1", SourceSHA256, SoilSnapshotSHA256,
		SoilSourceSQL("SoilClassGroup") + " " + SoilSourceSQL("SoilClassSubgroup"),
		[]ListDefinition{{"SoilClassGroup", 39}, {"SoilClassSubgroup", 62}}}
}
