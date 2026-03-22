package canonical

// X12 segment identifiers and element positions for the 850 Purchase Order
// transaction set. Positions are 1-based element indices within the segment.
const (
	// Interchange Control Header
	SegISA = "ISA"
	// ISA06 – sender ID (1-based index 6)
	ISASenderIdx = 6
	// ISA08 – receiver ID
	ISAReceiverIdx = 8
	// ISA13 – interchange control number
	ISAControlIdx = 13

	// Functional Group Header
	SegGS = "GS"

	// Transaction Set Header
	SegST  = "ST"
	ST850  = "850" // Purchase Order

	// Beginning segment for Purchase Order
	SegBEG = "BEG"
	// BEG03 – purchase order number
	BEGDocNumIdx = 3
	// BEG05 – purchase order date (YYYYMMDD)
	BEGOrderDateIdx = 5

	// Party Identification
	SegN1     = "N1"
	N1QualBY  = "BY" // buyer
	N1QualSE  = "SE" // seller
	N1NameIdx = 2
	N1IDIdx   = 4

	// Purchase Order Line Item
	SegPO1        = "PO1"
	PO1LineNumIdx = 1
	PO1QtyIdx     = 2
	PO1UoMIdx     = 3
	PO1PriceIdx   = 4
	// PO1 product code segments start at index 6, alternating qualifier/value
	PO1CodeStart = 6
	QualVP       = "VP" // vendor part
	QualBP       = "BP" // buyer part
	QualUP       = "UP" // UPC

	// Transaction Set Trailer
	SegSE  = "SE"
	SegGE  = "GE"
	SegIEA = "IEA"
)

// EDIFACT segment identifiers and element positions for ORDERS D.96A.
const (
	// Service String Advice
	SegUNA = "UNA"

	// Interchange Control Reference – UNB
	SegUNB        = "UNB"
	UNBSenderIdx  = 2 // UNB02 component 1
	UNBReceiverIdx = 3 // UNB03 component 1
	UNBDateIdx    = 4 // UNB04 component 1 (YYMMDD)
	UNBCtrlIdx    = 5 // UNB05

	// Message Reference Number
	SegUNH = "UNH"

	// Beginning of Message
	SegBGM     = "BGM"
	BGMDocIdx  = 2 // BGM02 – document number

	// Date/Time/Period
	SegDTM          = "DTM"
	DTMQual137      = "137" // document date
	DTMQual2        = 2     // value element within DTM01 composite
	DTMQual3        = 3     // format element within DTM01 composite

	// Name and Address
	SegNAD       = "NAD"
	NADQualBY    = "BY"
	NADQualSE    = "SE"
	NADQualSU    = "SU" // supplier (alternative to SE)
	NADIDIdx     = 2    // NAD02 – party identifier composite
	NADNameIdx   = 4    // NAD04 – party name (after ++ in compact form)

	// Line Item
	SegLIN       = "LIN"
	LINLineIdx   = 1 // LIN01 – line number
	LINItemIdx   = 3 // LIN03 composite: item+qualifier

	// Quantity
	SegQTY      = "QTY"
	QTYComposite = 1 // QTY01 composite: qualifier:qty:uom

	// Price Details
	SegPRI    = "PRI"
	PRIComposite = 1 // PRI01 composite: qualifier:price

	// Message Trailer
	SegUNT = "UNT"
	SegUNZ = "UNZ"
)
