import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, QRCode, Tag } from 'antd';
import { CopyOutlined, DownloadOutlined, QrcodeOutlined } from '@ant-design/icons';

interface SubLinksTabProps {
  subUrl: string;
  subClashUrl: string;
  onCopy: (value: string) => void;
}

const appendRawView = (url: string) => `${url}${url.includes('?') ? '&' : '?'}view=raw`;
const appendFlag = (url: string, flag: string) =>
  url ? `${url}${url.includes('?') ? '&' : '?'}flag=${flag}` : '';

export default function SubLinksTab({ subUrl, subClashUrl, onCopy }: SubLinksTabProps) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState<Record<string, boolean>>({
    CLASH: true,
    SHADOWROCKET: true,
  });

  const clashUrl = subClashUrl || appendFlag(subUrl, 'clash');
  const shadowrocketUrl = appendFlag(subUrl, 'shadowrocket');

  const rows = [
    { kind: 'CLASH', color: 'gold', url: clashUrl, title: 'Clash / Mihomo', downloadable: true },
    {
      kind: 'SHADOWROCKET',
      color: 'blue',
      url: shadowrocketUrl,
      title: 'Shadowrocket',
      downloadable: true,
    },
  ].filter((row) => row.url);

  const toggleQr = (kind: string) => {
    setExpanded((prev) => ({ ...prev, [kind]: !prev[kind] }));
  };

  return (
    <div className="sub-rows">
      {rows.map((row) => (
        <div key={row.kind} className="sub-row-card">
          <div className="sub-row">
            <Tag color={row.color} className="sub-row-tag">
              {row.kind}
            </Tag>
            <div className="sub-row-main">
              <a href={row.url} target="_blank" rel="noopener noreferrer" className="sub-row-title">
                {row.title}
              </a>
              <div className="sub-row-url" dir="ltr" title={row.url}>
                {row.url}
              </div>
            </div>
            <div className="sub-row-actions">
              {row.downloadable && (
                <Button
                  href={appendRawView(row.url)}
                  target="_blank"
                  rel="noopener noreferrer"
                  icon={<DownloadOutlined aria-hidden />}
                  aria-label={t('download')}
                  title={t('download')}
                />
              )}
              <Button
                icon={<CopyOutlined aria-hidden />}
                onClick={() => onCopy(row.url)}
                aria-label={t('copy')}
                title={t('copy')}
              />
              <Button
                type={expanded[row.kind] ? 'primary' : 'default'}
                icon={<QrcodeOutlined aria-hidden />}
                onClick={() => toggleQr(row.kind)}
                aria-label="QR"
                title="QR"
              />
            </div>
          </div>
          {expanded[row.kind] && (
            <div className="sub-row-qr-panel">
              <div className="sub-qr-code">
                <QRCode
                  value={row.url}
                  size={180}
                  type="svg"
                  bordered={false}
                  color="#000000"
                  bgColor="#ffffff"
                />
              </div>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
