Private Sub cmbEcosysCollectionStandard_Change()
Dim Answer As Variant
    If Left(Me.cmbEcosysCollectionStandard, 4) = "DEIF" Or Left(Me.cmbEcosysCollectionStandard, 3) = "DTE" Or Me.cmbEcosysCollectionStandard = "LMH25" Then
        Answer = MsgBox("VPro can populate some of these fields based on the selected collection standard.  Proceed?", vbYesNo, "VPro")
    End If
    If Answer = vbYes Then
        Me.CoordinatingAgency = "MoF"
        Me.ProponentFunder = "MoF"
        Me.FieldCompanyAgency = "Forests, Lands and Natural Resources"
        Me.GeographicStudyArea = "British Columbia"
        Me.VegCoverMethod = "Percent"
        Me.PlotMethod = "20x20"
        Me.GeoRefMethod = "GPS +/- 10m"
        Me.Datum = "NAD83"
        Me.CoordinateSystem = "dd.mm.ss.s"
        Me.CoverADescription = "Total of all tree layers (>10m)"
        Me.CoverA1Description = "Dominate trees"
        Me.CoverA2Description = "Main canopy"
        Me.CoverA3Description = "Trees > 10 m but below main canopy"
        Me.CoverBDescription = "Total of all shrub layers"
        Me.CoverB1Description = "Tall shrubs between 2 and 10 m tall"
        Me.CoverB2Description = "Low shrubs < 2 m tall"
        Me.CoverB2aDescription = "X"
        Me.CoverB2bDescription = "X"
        Me.CoverB2cDescription = "X"
        Me.CoverCDescription = "Herbaceous species and dwarf shrubs"
        Me.CoverDDescription = "Mosses, lichens, liverworts and seedlings"
        Me.Cover8Description = "Epixyls - species on downed wood"
        Me.Cover9Description = "Epiliths - species on rock"
        Me.Cover10Description = "Epiphytes - species on trees"
        Me.TableOfLists = TableOfListsVersion
        Me.AllSpecs = SpeciesListsVersion
        Me.DateLastEdited = Now()
    End If
End Sub
