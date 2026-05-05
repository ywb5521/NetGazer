import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useI18n } from '@/i18n/I18nContext';
import { useLocalStorage } from '@/lib/useLocalStorage';
import { formatBytes } from '@/lib/utils';
import { fetchCountryStats, fetchASNStats, type CountryStat, type ASNStat } from '@/lib/api';
import WorldMap from '@/components/geo/WorldMap';
import { WORLD_PATHS } from '@/components/geo/worldPaths';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import { Globe, ExternalLink } from 'lucide-react';

const COLORS = ['hsl(var(--chart-1))', 'hsl(var(--chart-2))', 'hsl(var(--chart-3))', 'hsl(var(--chart-4))', 'hsl(var(--chart-5))'];

function toISO(stat: CountryStat): string {
  return stat.iso_code || '';
}

export default function GeoPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [tab, setTab] = useLocalStorage('netgazer-geo-tab', 'map');
  const [countries, setCountries] = useState<CountryStat[]>([]);
  const [asns, setAsns] = useState<ASNStat[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const [c, a] = await Promise.all([
          fetchCountryStats().catch(() => [] as CountryStat[]),
          fetchASNStats().catch(() => [] as ASNStat[]),
        ]);
        if (!cancelled) {
          setCountries(Array.isArray(c) ? c : []);
          setAsns(Array.isArray(a) ? a : []);
        }
      } catch {
        if (!cancelled) {
          setCountries([]);
          setAsns([]);
        }
      }
      if (!cancelled) setLoading(false);
    };
    load();
    return () => { cancelled = true; };
  }, []);

  const mapData = useMemo(() =>
    countries
      .filter((c) => {
        const iso = (c.iso_code || '').toUpperCase();
        // Only include real countries with 2-letter ISO codes that exist on the map
        return iso.length === 2 && iso in WORLD_PATHS;
      })
      .map((c) => ({
        code: toISO(c).toUpperCase(),
        name: c.country,
        bytes: c.bytes,
        percentage: c.percentage,
      })),
    [countries],
  );

  const topCountries = useMemo(() => countries.slice(0, 15), [countries]);
  const topASNs = useMemo(() => asns.slice(0, 15), [asns]);

  // Build GeoIP name lookup from ISO code
  const geoNameByCode = useMemo(() => {
    const m: Record<string, string> = {};
    for (const c of countries) {
      const iso = (c.iso_code || '').toUpperCase();
      if (iso.length === 2 && c.country) {
        m[iso] = c.country;
      }
    }
    return m;
  }, [countries]);

  const handleCountryClick = (code: string, _mapName: string) => {
    // Use GeoIP country name for accurate backend filtering
    const geoName = geoNameByCode[code.toUpperCase()] || _mapName;
    navigate(`/hosts?country=${encodeURIComponent(geoName)}`);
  };

  const handleASNClick = (asn: ASNStat) => {
    navigate(`/hosts?asn=${encodeURIComponent(asn.org || asn.asn)}`);
  };

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
        <Globe className="h-6 w-6" />
        {t.geo.title}
      </h1>

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList>
          <TabsTrigger value="map">{t.geo.map}</TabsTrigger>
          <TabsTrigger value="countries">{t.geo.countries}</TabsTrigger>
          <TabsTrigger value="asns">{t.geo.asns}</TabsTrigger>
        </TabsList>

        <TabsContent value="map">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <Card className="lg:col-span-2">
              <CardContent className="pt-6">
                {countries.length > 0 ? (
                  <div className="w-full aspect-[2]">
                    <WorldMap data={mapData} onCountryClick={handleCountryClick} />
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground py-16 text-center">
                    {loading ? t.geo.loading : t.geo.empty}
                  </p>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">{t.geo.countries}</CardTitle>
              </CardHeader>
              <CardContent>
                {countries.length > 0 ? (
                  <div className="space-y-0.5 max-h-[440px] overflow-auto">
                    {countries.slice(0, 15).map((c) => (
                      <div
                        key={c.country}
                        className="flex items-center justify-between text-xs p-2 rounded-md hover:bg-accent cursor-pointer transition-colors group"
                        onClick={() => handleCountryClick('', c.country)}
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <div className="h-2.5 w-2.5 rounded-full shrink-0" style={{ backgroundColor: COLORS[Math.abs(c.country.length) % COLORS.length] }} />
                          <span className="font-medium truncate max-w-[110px]">{c.country}</span>
                          <span className="text-muted-foreground">{c.hosts} {t.geo.hosts}</span>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          <div className="h-1.5 w-14 rounded-full bg-muted overflow-hidden">
                            <div className="h-full rounded-full bg-primary" style={{ width: `${Math.min(c.percentage, 100)}%` }} />
                          </div>
                          <span className="text-muted-foreground w-12 text-right">{c.percentage.toFixed(1)}%</span>
                          <ExternalLink className="h-3 w-3 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground py-8 text-center">{t.geo.empty}</p>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="countries">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">{t.geo.countries}</CardTitle>
              </CardHeader>
              <CardContent>
                {topCountries.length > 0 ? (
                  <div className="h-[400px]">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={topCountries} layout="vertical" margin={{ left: 60, right: 20, top: 5, bottom: 5 }}>
                        <XAxis type="number" tick={{ fontSize: 11 }} tickFormatter={(v) => formatBytes(v)} />
                        <YAxis type="category" dataKey="country" tick={{ fontSize: 11 }} width={100} />
                        <Tooltip formatter={(v: number) => formatBytes(v)} />
                        <Bar dataKey="bytes" radius={[0, 4, 4, 0]} cursor="pointer">
                          {topCountries.map((_, i) => (
                            <Cell key={i} fill={COLORS[i % COLORS.length]} />
                          ))}
                        </Bar>
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground py-16 text-center">
                    {loading ? t.geo.loading : t.geo.empty}
                  </p>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t.geo.country}</TableHead>
                      <TableHead className="text-right">{t.geo.hosts}</TableHead>
                      <TableHead className="text-right">{t.geo.bytes}</TableHead>
                      <TableHead className="text-right">{t.geo.percentage}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {topCountries.map((c) => (
                      <TableRow
                        key={c.country}
                        className="cursor-pointer hover:bg-accent"
                        onClick={() => handleCountryClick('', c.country)}
                      >
                        <TableCell className="text-sm font-medium flex items-center gap-2">
                          <ExternalLink className="h-3 w-3 text-muted-foreground opacity-0 group-hover:opacity-100" />
                          {c.country}
                        </TableCell>
                        <TableCell className="text-right text-xs">{c.hosts}</TableCell>
                        <TableCell className="text-right text-xs">{formatBytes(c.bytes)}</TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-2">
                            <div className="h-2 w-16 rounded-full bg-muted overflow-hidden">
                              <div className="h-full rounded-full bg-chart-1" style={{ width: `${c.percentage}%` }} />
                            </div>
                            <span className="text-xs text-muted-foreground">{c.percentage.toFixed(1)}%</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                    {topCountries.length === 0 && (
                      <TableRow>
                        <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                          {loading ? t.geo.loading : t.geo.empty}
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="asns">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">{t.geo.asns}</CardTitle>
              </CardHeader>
              <CardContent>
                {topASNs.length > 0 ? (
                  <div className="h-[400px]">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={topASNs} layout="vertical" margin={{ left: 80, right: 20, top: 5, bottom: 5 }}>
                        <XAxis type="number" tick={{ fontSize: 11 }} tickFormatter={(v) => formatBytes(v)} />
                        <YAxis type="category" dataKey="org" tick={{ fontSize: 11 }} width={130} />
                        <Tooltip formatter={(v: number) => formatBytes(v)} />
                        <Bar dataKey="bytes" radius={[0, 4, 4, 0]} cursor="pointer">
                          {topASNs.map((_, i) => (
                            <Cell key={i} fill={COLORS[i % COLORS.length]} />
                          ))}
                        </Bar>
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground py-16 text-center">
                    {loading ? t.geo.loading : t.geo.empty}
                  </p>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>ASN</TableHead>
                      <TableHead className="text-right">{t.geo.hosts}</TableHead>
                      <TableHead className="text-right">{t.geo.bytes}</TableHead>
                      <TableHead className="text-right">{t.geo.percentage}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {topASNs.map((a) => (
                      <TableRow
                        key={a.asn}
                        className="cursor-pointer hover:bg-accent"
                        onClick={() => handleASNClick(a)}
                      >
                        <TableCell className="text-sm font-medium max-w-[180px] truncate" title={a.asn}>{a.asn}</TableCell>
                        <TableCell className="text-right text-xs">{a.hosts}</TableCell>
                        <TableCell className="text-right text-xs">{formatBytes(a.bytes)}</TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-2">
                            <div className="h-2 w-16 rounded-full bg-muted overflow-hidden">
                              <div className="h-full rounded-full bg-chart-1" style={{ width: `${a.percentage}%` }} />
                            </div>
                            <span className="text-xs text-muted-foreground">{a.percentage.toFixed(1)}%</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                    {topASNs.length === 0 && (
                      <TableRow>
                        <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                          {loading ? t.geo.loading : t.geo.empty}
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
