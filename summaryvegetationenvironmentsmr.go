package main

func summaryVegetationSMRClassCode(code string) string {
	switch code {
	case "0":
		return "VX"
	case "1":
		return "X"
	case "2":
		return "SX"
	case "3":
		return "SM"
	case "4":
		return "M"
	case "5":
		return "SHG"
	case "6":
		return "HG"
	case "7":
		return "SHD"
	case "8":
		return "HD"
	default:
		return ""
	}
}
