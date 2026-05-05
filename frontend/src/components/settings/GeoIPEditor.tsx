import { useEffect, useRef, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { useI18n } from '@/i18n/I18nContext';
import { fetchGeoIPStatus, uploadGeoIPDB, downloadGeoIPDB, type GeoIPStatus } from '@/lib/api';
import { Globe, Upload, Download, CheckCircle, XCircle } from 'lucide-react';

export function GeoIPEditor() {
  const { t } = useI18n();
  const [status, setStatus] = useState<GeoIPStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState<'country' | 'asn' | null>(null);
  const [downloadUrl, setDownloadUrl] = useState('');
  const [downloadType, setDownloadType] = useState<'country' | 'asn'>('country');
  const [downloading, setDownloading] = useState(false);
  const countryInputRef = useRef<HTMLInputElement>(null);
  const asnInputRef = useRef<HTMLInputElement>(null);

  const loadStatus = async () => {
    try {
      const s = await fetchGeoIPStatus();
      setStatus(s);
    } catch {
      setStatus(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadStatus(); }, []);

  const handleUpload = async (type: 'country' | 'asn') => {
    const inputRef = type === 'country' ? countryInputRef : asnInputRef;
    const file = inputRef.current?.files?.[0];
    if (!file) return;

    setUploading(type);
    try {
      await uploadGeoIPDB(file, type);
      await loadStatus();
    } finally {
      setUploading(null);
    }
  };

  const handleDownload = async () => {
    if (!downloadUrl.trim()) return;
    setDownloading(true);
    try {
      await downloadGeoIPDB(downloadUrl.trim(), downloadType);
      await loadStatus();
      setDownloadUrl('');
    } finally {
      setDownloading(false);
    }
  };

  if (loading) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm flex items-center gap-2">
          <Globe className="h-4 w-4" />
          {t.settings.geoip}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <p className="text-sm text-muted-foreground">{t.settings.geoipDesc}</p>

        {/* Status */}
        <div className="rounded-md border p-3 space-y-1 text-sm">
          <div className="font-medium text-xs text-muted-foreground uppercase tracking-wider">{t.settings.geoipStatus}</div>
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground w-32">{t.settings.geoipCountryDb}:</span>
            {status?.country_db ? (
              <Badge variant="outline" className="gap-1">
                <CheckCircle className="h-3 w-3 text-green-500" />
                {status.country_db.split('/').pop()}
              </Badge>
            ) : (
              <Badge variant="secondary" className="gap-1">
                <XCircle className="h-3 w-3" />
                {t.settings.geoipNotLoaded}
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground w-32">{t.settings.geoipAsnDb}:</span>
            {status?.asn_db ? (
              <Badge variant="outline" className="gap-1">
                <CheckCircle className="h-3 w-3 text-green-500" />
                {status.asn_db.split('/').pop()}
              </Badge>
            ) : (
              <Badge variant="secondary" className="gap-1">
                <XCircle className="h-3 w-3" />
                {t.settings.geoipNotLoaded}
              </Badge>
            )}
          </div>
          {status?.country_info && (
            <div className="text-xs text-muted-foreground pl-32">{status.country_info}</div>
          )}
          {status?.asn_info && (
            <div className="text-xs text-muted-foreground pl-32">{status.asn_info}</div>
          )}
        </div>

        {/* Upload Country */}
        <div className="space-y-2">
          <label className="text-sm font-medium">{t.settings.geoipCountryDb}</label>
          <div className="flex gap-2">
            <Input ref={countryInputRef} type="file" accept=".mmdb" className="flex-1" />
            <Button variant="outline" size="sm" disabled={uploading === 'country'} onClick={() => handleUpload('country')}>
              <Upload className="h-4 w-4 mr-1" />
              {uploading === 'country' ? '...' : t.settings.geoipUpload}
            </Button>
          </div>
        </div>

        {/* Upload ASN */}
        <div className="space-y-2">
          <label className="text-sm font-medium">{t.settings.geoipAsnDb}</label>
          <div className="flex gap-2">
            <Input ref={asnInputRef} type="file" accept=".mmdb" className="flex-1" />
            <Button variant="outline" size="sm" disabled={uploading === 'asn'} onClick={() => handleUpload('asn')}>
              <Upload className="h-4 w-4 mr-1" />
              {uploading === 'asn' ? '...' : t.settings.geoipUpload}
            </Button>
          </div>
        </div>

        {/* Download from URL */}
        <div className="space-y-2 border-t pt-4">
          <label className="text-sm font-medium">{t.settings.geoipDownloadUrl}</label>
          <div className="flex gap-2 items-center">
            <select
              className="border rounded-md px-2 py-1.5 text-sm bg-background"
              value={downloadType}
              onChange={(e) => setDownloadType(e.target.value as 'country' | 'asn')}
            >
              <option value="country">{t.settings.geoipCountryDb}</option>
              <option value="asn">{t.settings.geoipAsnDb}</option>
            </select>
            <Input
              placeholder="https://example.com/GeoLite2-Country.mmdb"
              value={downloadUrl}
              onChange={(e) => setDownloadUrl(e.target.value)}
              className="flex-1"
            />
            <Button variant="outline" size="sm" disabled={downloading || !downloadUrl.trim()} onClick={handleDownload}>
              <Download className="h-4 w-4 mr-1" />
              {downloading ? t.settings.geoipDownloading : t.settings.geoipDownload}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
