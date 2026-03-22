#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# EDI Simulator – realistic population & end-to-end exercise script
# Usage: bash scripts/populate.sh
# -----------------------------------------------------------------------------
set -euo pipefail

BASE="http://localhost:8080/api/v1"
SEP="────────────────────────────────────────────────────────"

ok()  { echo "  ✓  $*"; }
hdr() { echo; echo "$SEP"; echo "  $*"; echo "$SEP"; }
err() { echo "  ✗  $*" >&2; }

post() { curl -s -X POST "$BASE/$1" -H "Content-Type: application/json" -d "$2"; }
get()  { curl -s "$BASE/$1"; }

# ── 1. Health ─────────────────────────────────────────────────────────────────
hdr "1 · Health check"
get "health" | jq .
ok "API gateway is up"

# ── 2. Create realistic messages ──────────────────────────────────────────────
hdr "2 · Creating messages"

# ── X12 850 Purchase Order ─────────────────────────────────────────────────
X12_850='ISA*00*          *00*          *ZZ*AUTOPARTS-CO    *ZZ*SUPPLIER-CORP   *260301*0900*^*00501*000000101*0*P*:~
GS*PO*AUTOPARTS-CO*SUPPLIER-CORP*20260301*0900*101*X*005010~
ST*850*0001~
BEG*00*SA*PO-2026-0301-001**20260301~
CUR*BY*USD~
N1*BY*Autoparts Co.*92*APC001~
N1*SE*Supplier Corp*92*SC002~
N3*1000 Industrial Blvd~
N4*Detroit*MI*48201*US~
PO1*1*500*EA*12.50**PI*BRAKE-PAD-A1*VP*BP-GOLD-500~
PID*F****Premium Brake Pad - Gold Series~
PO1*2*250*EA*8.99**PI*FILTER-OIL-K5*VP*FO-K5-BULK~
PID*F****Oil Filter K5 - Bulk Pack~
PO1*3*100*EA*45.00**PI*SHOCK-ABSORBER-R*VP*SA-R200~
PID*F****Rear Shock Absorber R200~
CTT*3*850~
AMT*1*15472.50~
SE*17*0001~
GE*1*101~
IEA*1*000000101~'

X12_810='ISA*00*          *00*          *ZZ*SUPPLIER-CORP   *ZZ*AUTOPARTS-CO    *260301*1030*^*00501*000000102*0*P*:~
GS*IN*SUPPLIER-CORP*AUTOPARTS-CO*20260301*1030*102*X*005010~
ST*810*0001~
BIG*20260301*INV-2026-0301-042**PO-2026-0228-099~
N1*SE*Supplier Corp*92*SC002~
N1*BY*Autoparts Co.*92*APC001~
IT1*1*500*EA*12.50**PI*BRAKE-PAD-A1~
IT1*2*250*EA*8.99**PI*FILTER-OIL-K5~
TDS*1572250~
TXI*TX*7.5~
CAD*M**UPS*1Z9999990123456784~
SE*13*0001~
GE*1*102~
IEA*1*000000102~'

X12_856='ISA*00*          *00*          *ZZ*SUPPLIER-CORP   *ZZ*AUTOPARTS-CO    *260301*1400*^*00501*000000103*0*P*:~
GS*SH*SUPPLIER-CORP*AUTOPARTS-CO*20260301*1400*103*X*005010~
ST*856*0001~
BSN*00*SHIP-2026-0301-001*20260301*1400~
HL*1**S~
TD5*B*2*UPS**ZZ~
N1*SE*Supplier Corp*92*SC002~
N1*BY*Autoparts Co.*92*APC001~
HL*2*1*O~
PRF*PO-2026-0228-099~
HL*3*2*P~
PO4*1*1*CTN~
HL*4*3*I~
LIN**PI*BRAKE-PAD-A1~
SN1*1*500*EA~
SE*15*0001~
GE*1*103~
IEA*1*000000103~'

# ── EDIFACT ORDERS ─────────────────────────────────────────────────────────
EDIFACT_ORDERS="UNA:+.? '
UNB+UNOA:2+RETAILER-EU:1+EUROPARTS:1+260301:0800+000201++ORDERS'
UNH+1+ORDERS:D:01B:UN:EAN010'
BGM+220+ORD-EU-2026-0301-001+9'
DTM+137:20260301:102'
DTM+2:20260310:102'
NAD+BY+++RetailEU Sarl+12 Rue du Commerce+Paris++75001+FR'
NAD+SE+++EuroParts GmbH+Industriestrasse 44+Munich++80339+DE'
CUX+2:EUR:4'
LIN+1++CALIPER-FR-400:IN'
QTY+21:200'
PRI+AAA:38.50:CA'
LIN+2++DISC-BRAKE-R55:IN'
QTY+21:150'
PRI+AAA:72.00:CA'
LIN+3++CLUTCH-KIT-CK9:IN'
QTY+21:80'
PRI+AAA:125.00:CA'
UNS+S'
CNT+2:3'
MOA+86:29650.00'
UNT+20+1'
UNZ+1+000201'"

EDIFACT_INVOIC="UNA:+.? '
UNB+UNOA:2+EUROPARTS:1+RETAILER-EU:1+260301:1100+000202++INVOIC'
UNH+1+INVOIC:D:01B:UN:EAN010'
BGM+380+INV-EU-2026-0301-007+9'
DTM+137:20260301:102'
DTM+35:20260331:102'
NAD+SE+++EuroParts GmbH+Industriestrasse 44+Munich++80339+DE'
NAD+BY+++RetailEU Sarl+12 Rue du Commerce+Paris++75001+FR'
CUX+2:EUR:4'
LIN+1++CALIPER-FR-400:IN'
QTY+47:200'
PRI+AAA:38.50:CA'
MOA+203:7700.00'
LIN+2++DISC-BRAKE-R55:IN'
QTY+47:150'
PRI+AAA:72.00:CA'
MOA+203:10800.00'
UNS+S'
MOA+86:18500.00'
TAX+7+VAT+++:::20'
MOA+124:3700.00'
MOA+9:22200.00'
UNT+22+1'
UNZ+1+000202'"

# ── XML Purchase Order ──────────────────────────────────────────────────────
XML_PO='<Document>
  <Envelope>
    <Sender>ASIAPAC-MOTORS</Sender>
    <Receiver>GLOBAL-AUTO-SUPPLY</Receiver>
    <ControlNumber>PO-APAC-2026-0301-001</ControlNumber>
  </Envelope>
  <Body>
    <DocumentNumber>PO-APAC-2026-0301-001</DocumentNumber>
    <OrderDate>2026-03-01T09:00:00Z</OrderDate>
    <Currency>USD</Currency>
    <BuyerParty>
      <ID>APM-001</ID>
      <Name>AsiaPac Motors Ltd</Name>
      <Street>88 Harbour Road</Street>
      <City>Hong Kong</City>
      <Country>HK</Country>
    </BuyerParty>
    <SellerParty>
      <ID>GAS-002</ID>
      <Name>Global Auto Supply Inc</Name>
      <Street>200 Commerce Drive</Street>
      <City>Chicago</City>
      <State>IL</State>
      <Country>US</Country>
    </SellerParty>
    <LineItems>
      <LineItem>
        <LineNumber>1</LineNumber>
        <PartNumber>SENSOR-O2-V6</PartNumber>
        <Description>Oxygen Sensor V6 Engine Compatible</Description>
        <Quantity>300</Quantity>
        <UnitPrice>22.50</UnitPrice>
        <UOM>EA</UOM>
      </LineItem>
      <LineItem>
        <LineNumber>2</LineNumber>
        <PartNumber>ECU-MODULE-GX5</PartNumber>
        <Description>Engine Control Unit GX5 Series</Description>
        <Quantity>50</Quantity>
        <UnitPrice>185.00</UnitPrice>
        <UOM>EA</UOM>
      </LineItem>
      <LineItem>
        <LineNumber>3</LineNumber>
        <PartNumber>TRANS-FLUID-ATF</PartNumber>
        <Description>Automatic Transmission Fluid ATF-4</Description>
        <Quantity>1000</Quantity>
        <UnitPrice>4.75</UnitPrice>
        <UOM>QT</UOM>
      </LineItem>
    </LineItems>
  </Body>
</Document>'

XML_INVOICE='<Document>
  <Envelope>
    <Sender>GLOBAL-AUTO-SUPPLY</Sender>
    <Receiver>ASIAPAC-MOTORS</Receiver>
    <ControlNumber>INV-APAC-2026-0301-042</ControlNumber>
  </Envelope>
  <Body>
    <DocumentNumber>INV-APAC-2026-0301-042</DocumentNumber>
    <OrderDate>2026-03-01T11:00:00Z</OrderDate>
    <Currency>USD</Currency>
    <SellerParty>
      <ID>GAS-002</ID>
      <Name>Global Auto Supply Inc</Name>
    </SellerParty>
    <BuyerParty>
      <ID>APM-001</ID>
      <Name>AsiaPac Motors Ltd</Name>
    </BuyerParty>
    <LineItems>
      <LineItem>
        <LineNumber>1</LineNumber>
        <PartNumber>SENSOR-O2-V6</PartNumber>
        <Quantity>300</Quantity>
        <UnitPrice>22.50</UnitPrice>
        <UOM>EA</UOM>
      </LineItem>
      <LineItem>
        <LineNumber>2</LineNumber>
        <PartNumber>ECU-MODULE-GX5</PartNumber>
        <Quantity>50</Quantity>
        <UnitPrice>185.00</UnitPrice>
        <UOM>EA</UOM>
      </LineItem>
    </LineItems>
  </Body>
</Document>'

declare -A MSG_IDS

create_msg() {
  local name="$1" format="$2" content="$3" sender="$4" receiver="$5"
  local escaped
  escaped=$(printf '%s' "$content" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')
  local payload="{\"format\":\"${format}\",\"content\":${escaped},\"sender\":\"${sender}\",\"receiver\":\"${receiver}\"}"
  local id
  id=$(post "messages" "$payload" | jq -r '.id')
  MSG_IDS["$name"]="$id"
  ok "$name → $id"
}

create_msg "x12_850"         "x12"     "$X12_850"         "AUTOPARTS-CO"  "SUPPLIER-CORP"
create_msg "x12_810"         "x12"     "$X12_810"         "SUPPLIER-CORP" "AUTOPARTS-CO"
create_msg "x12_856"         "x12"     "$X12_856"         "SUPPLIER-CORP" "AUTOPARTS-CO"
create_msg "edifact_orders"  "edifact" "$EDIFACT_ORDERS"  "RETAILER-EU"   "EUROPARTS"
create_msg "edifact_invoic"  "edifact" "$EDIFACT_INVOIC"  "EUROPARTS"     "RETAILER-EU"
create_msg "xml_po"          "xml"     "$XML_PO"          "ASIAPAC-MOTORS" "GLOBAL-AUTO-SUPPLY"
create_msg "xml_invoice"     "xml"     "$XML_INVOICE"     "GLOBAL-AUTO-SUPPLY" "ASIAPAC-MOTORS"

# ── 3. Wait for pipeline to process ───────────────────────────────────────────
hdr "3 · Waiting for pipeline (worker → sender → receiver)…"
sleep 5

# ── 4. Check statuses ─────────────────────────────────────────────────────────
hdr "4 · Message statuses after pipeline"
get "messages" | jq '[.[] | {id: .id[0:8], format: .format, sender: .sender, status: .status}]'

# ── 5. Stateful transform: X12 850 → EDIFACT ─────────────────────────────────
hdr "5 · Stateful transform: X12 850 PO → EDIFACT"
ID="${MSG_IDS[x12_850]}"
RESULT=$(post "transform" "{\"message_id\":\"${ID}\",\"source_format\":\"x12\",\"target_format\":\"edifact\"}")
echo "$RESULT" | jq '{message_id: .message_id, target_format: .target_format, canonical_stored: .canonical_stored}'
echo "--- EDIFACT output (first 400 chars) ---"
echo "$RESULT" | jq -r '.result' | head -c 400
echo

# ── 6. Stateful transform: X12 810 → XML ─────────────────────────────────────
hdr "6 · Stateful transform: X12 810 Invoice → XML"
ID="${MSG_IDS[x12_810]}"
RESULT=$(post "transform" "{\"message_id\":\"${ID}\",\"source_format\":\"x12\",\"target_format\":\"xml\"}")
echo "$RESULT" | jq '{message_id: .message_id, target_format: .target_format, canonical_stored: .canonical_stored}'
echo "--- XML output (first 400 chars) ---"
echo "$RESULT" | jq -r '.result' | head -c 400
echo

# ── 7. Stateful transform: EDIFACT ORDERS → X12 ──────────────────────────────
hdr "7 · Stateful transform: EDIFACT ORDERS → X12"
ID="${MSG_IDS[edifact_orders]}"
RESULT=$(post "transform" "{\"message_id\":\"${ID}\",\"source_format\":\"edifact\",\"target_format\":\"x12\"}")
echo "$RESULT" | jq '{message_id: .message_id, target_format: .target_format, canonical_stored: .canonical_stored}'
echo "--- X12 output (first 400 chars) ---"
echo "$RESULT" | jq -r '.result' | head -c 400
echo

# ── 8. Stateful transform: XML PO → EDIFACT ──────────────────────────────────
hdr "8 · Stateful transform: XML PO → EDIFACT"
ID="${MSG_IDS[xml_po]}"
RESULT=$(post "transform" "{\"message_id\":\"${ID}\",\"source_format\":\"xml\",\"target_format\":\"edifact\"}")
echo "$RESULT" | jq '{message_id: .message_id, target_format: .target_format, canonical_stored: .canonical_stored}'
echo "--- EDIFACT output (first 400 chars) ---"
echo "$RESULT" | jq -r '.result' | head -c 400
echo

# ── 9. Preview (stateless) ─────────────────────────────────────────────────────
hdr "9 · Preview (stateless): EDIFACT INVOIC → XML"
RESULT=$(post "transform/preview" "{\"source_format\":\"edifact\",\"target_format\":\"xml\",\"content\":$(printf '%s' "$EDIFACT_INVOIC" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')}")
echo "$RESULT" | jq '{source_format, target_format, fields_mapped}'
echo "--- Canonical XML (first 500 chars) ---"
echo "$RESULT" | jq -r '.canonical' | head -c 500
echo

# ── 10. Audit trail ───────────────────────────────────────────────────────────
hdr "10 · Audit trail for X12 850 PO"
get "messages/${MSG_IDS[x12_850]}/transactions" | jq '[.[] | {event: .event, timestamp: .timestamp}]'

# ── 11. Queue status ──────────────────────────────────────────────────────────
hdr "11 · Queue status"
get "queue/status" | jq '.queues[] | {name: .name, messages: .messages, consumers: .consumers}'

# ── 12. DLQ status ────────────────────────────────────────────────────────────
hdr "12 · DLQ status (send + receive)"
get "queue/dlq?type=messages" | jq .
get "queue/dlq?type=messages.messages" | jq . 2>/dev/null || true

# ── 13. Final status roll-up ──────────────────────────────────────────────────
hdr "13 · Final status roll-up"
get "messages" | jq 'group_by(.status) | map({status: .[0].status, count: length})'

echo
echo "$SEP"
echo "  All done. 7 messages created and exercised."
echo "$SEP"
