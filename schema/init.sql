-- Create messages table
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    format VARCHAR(50) NOT NULL DEFAULT 'X12',
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    sender VARCHAR(255),
    receiver VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'received', 'processed', 'failed', 'transformed')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP DEFAULT NULL  -- Tracks when message was published to queue
);

-- Create transactions audit table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    event VARCHAR(100) NOT NULL,
    details JSONB DEFAULT '{}'::jsonb,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
CREATE INDEX IF NOT EXISTS idx_messages_format ON messages(format);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_pending_unpublished ON messages(status, published_at) WHERE status='pending' AND published_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_transactions_message_id ON transactions(message_id);
CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(timestamp DESC);

-- Transformation mappings: registry of supported format-pair routes
CREATE TABLE IF NOT EXISTS transformation_mappings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    source_format   TEXT NOT NULL,
    target_format   TEXT NOT NULL,
    active          BOOLEAN NOT NULL DEFAULT true,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mappings_source ON transformation_mappings(source_format);
CREATE INDEX IF NOT EXISTS idx_mappings_target ON transformation_mappings(target_format);
CREATE INDEX IF NOT EXISTS idx_mappings_active  ON transformation_mappings(active);

INSERT INTO transformation_mappings (name, source_format, target_format, description)
SELECT name, source_format, target_format, description FROM (VALUES
  ('X12 -> EDIFACT', 'x12',     'edifact', 'Purchase Order: X12 to UN/EDIFACT'),
  ('EDIFACT -> X12', 'edifact', 'x12',     'Purchase Order: UN/EDIFACT to X12'),
  ('X12 -> XML',     'x12',     'xml',     'Purchase Order: X12 to Generic XML'),
  ('XML -> X12',     'xml',     'x12',     'Purchase Order: Generic XML to X12'),
  ('EDIFACT -> XML', 'edifact', 'xml',     'Purchase Order: EDIFACT to Generic XML'),
  ('XML -> EDIFACT', 'xml',     'edifact', 'Purchase Order: Generic XML to UN/EDIFACT')
) AS v(name, source_format, target_format, description)
WHERE NOT EXISTS (SELECT 1 FROM transformation_mappings LIMIT 1);

-- Trading partners: simulated companies for EDI send/receive
CREATE TABLE IF NOT EXISTS trading_partners (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    country         CHAR(2) NOT NULL DEFAULT 'US',
    preferred_format TEXT NOT NULL CHECK (preferred_format IN ('x12', 'edifact', 'xml')),
    edi_qualifier   TEXT NOT NULL DEFAULT 'ZZ',
    edi_id          TEXT NOT NULL,
    active          BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_partners_format ON trading_partners(preferred_format);
CREATE INDEX IF NOT EXISTS idx_partners_active  ON trading_partners(active);

INSERT INTO trading_partners (name, country, preferred_format, edi_qualifier, edi_id)
SELECT name, country, preferred_format, edi_qualifier, edi_id FROM (VALUES
  ('Autoparts Co',        'US', 'x12',     '01', 'AUTOPARTS01'),
  ('Supplier Corp',       'US', 'x12',     '01', 'SUPPLIER01'),
  ('EuroParts GmbH',      'DE', 'edifact', '14', 'EUROPARTS01'),
  ('RetailEU SARL',       'FR', 'edifact', '14', 'RETAILEU01'),
  ('AsiaPac Motors',      'HK', 'xml',     'ZZ', 'ASIAPAC01'),
  ('Global Auto Supply',  'US', 'xml',     'ZZ', 'GLOBALAUTO01'),
  ('NordikParts AS',      'NO', 'edifact', '14', 'NORDIKP01'),
  ('Pacific Logistics Co','AU', 'x12',     '01', 'PACLOGIS01')
) AS v(name, country, preferred_format, edi_qualifier, edi_id)
WHERE NOT EXISTS (SELECT 1 FROM trading_partners LIMIT 1);

-- LLM async job queue
CREATE TABLE IF NOT EXISTS llm_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type         VARCHAR(50)  NOT NULL,
    input_ref    UUID,
    status       VARCHAR(20)  NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','running','done','error','timeout')),
    result       JSONB,
    error        TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_llm_jobs_status    ON llm_jobs(status);
CREATE INDEX IF NOT EXISTS idx_llm_jobs_input_ref ON llm_jobs(input_ref);
