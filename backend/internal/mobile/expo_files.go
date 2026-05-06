package mobile

import (
	"encoding/json"
	"fmt"
)

func rootLayoutTSX() string {
	return `import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Stack } from 'expo-router';
import { StatusBar } from 'react-native';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { AuthProvider } from '@/auth/AuthProvider';
import { theme } from '@/theme/theme';

const queryClient = new QueryClient();

export default function RootLayout() {
  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <StatusBar barStyle="light-content" backgroundColor={theme.colors.navy} />
          <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: theme.colors.navy } }}>
            <Stack.Screen name="index" />
            <Stack.Screen name="(auth)/login" />
            <Stack.Screen name="(tabs)" />
            <Stack.Screen name="modals/estimate-detail" options={{ presentation: 'modal' }} />
          </Stack>
        </AuthProvider>
      </QueryClientProvider>
    </SafeAreaProvider>
  );
}
`
}

func indexTSX() string {
	return `import { Redirect } from 'expo-router';
import { useAuth } from '@/auth/AuthProvider';

export default function Index() {
  const { session, bootstrapped } = useAuth();
  if (!bootstrapped) return null;
  return <Redirect href={session ? '/(tabs)/jobs' : '/(auth)/login'} />;
}
`
}

func loginTSX() string {
	return `import { useState } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';
import { router } from 'expo-router';
import { useAuth } from '@/auth/AuthProvider';
import { Screen } from '@/components/ui/Screen';
import { theme } from '@/theme/theme';

export default function LoginScreen() {
  const { signIn } = useAuth();
  const [email, setEmail] = useState('owner@fieldops.test');
  const [password, setPassword] = useState('demo-password');
  const [error, setError] = useState('');

  const submit = async () => {
    if (!email.includes('@') || password.length < 6) {
      setError('Enter a valid email and a password with at least 6 characters.');
      return;
    }
    await signIn(email, password);
    router.replace('/(tabs)/jobs');
  };

  return (
    <Screen title="FieldOps Login" subtitle="Secure field access for crews and estimators.">
      <View style={{ gap: 14 }}>
        <TextInput
          accessibilityLabel="Email address"
          autoCapitalize="none"
          keyboardType="email-address"
          value={email}
          onChangeText={setEmail}
          style={theme.input}
          placeholder="owner@company.com"
          placeholderTextColor={theme.colors.muted}
        />
        <TextInput
          accessibilityLabel="Password"
          secureTextEntry
          value={password}
          onChangeText={setPassword}
          style={theme.input}
          placeholder="Password"
          placeholderTextColor={theme.colors.muted}
        />
        {error ? <Text style={{ color: theme.colors.danger }}>{error}</Text> : null}
        <Pressable accessibilityRole="button" onPress={submit} style={theme.primaryButton}>
          <Text style={theme.primaryButtonText}>Sign in</Text>
        </Pressable>
      </View>
    </Screen>
  );
}
`
}

func tabsLayoutTSX() string {
	return `import { Tabs } from 'expo-router';
import { theme } from '@/theme/theme';

export default function TabsLayout() {
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: theme.colors.cyan,
        tabBarInactiveTintColor: theme.colors.muted,
        tabBarStyle: { backgroundColor: theme.colors.panel, borderTopColor: theme.colors.border }
      }}
    >
      <Tabs.Screen name="jobs" options={{ title: 'Jobs' }} />
      <Tabs.Screen name="customers" options={{ title: 'Customers' }} />
      <Tabs.Screen name="estimates" options={{ title: 'Estimates' }} />
    </Tabs>
  );
}
`
}

func jobsScreenTSX() string {
	return `import { FlatList, Pressable, Text, View } from 'react-native';
import { router } from 'expo-router';
import { Screen } from '@/components/ui/Screen';
import { jobs } from '@/features/fieldService/sampleData';
import { theme } from '@/theme/theme';

export default function JobsScreen() {
  return (
    <Screen title="Active Jobs" subtitle="Live field pipeline with offline-safe draft state." scroll={false}>
      <FlatList
        data={jobs}
        keyExtractor={(item) => item.id}
        contentContainerStyle={{ gap: 12 }}
        renderItem={({ item }) => (
          <Pressable accessibilityRole="button" onPress={() => router.push('/modals/estimate-detail')} style={theme.card}>
            <View style={{ flexDirection: 'row', justifyContent: 'space-between', gap: 12 }}>
              <Text style={theme.cardTitle}>{item.title}</Text>
              <Text style={theme.badge}>{item.status}</Text>
            </View>
            <Text style={theme.body}>{item.customerName} • {item.address}</Text>
            <Text style={theme.caption}>Quote target: ${item.estimatedValue.toLocaleString()} • {item.syncState}</Text>
          </Pressable>
        )}
      />
    </Screen>
  );
}
`
}

func customersScreenTSX() string {
	return `import { FlatList, Text, View } from 'react-native';
import { Screen } from '@/components/ui/Screen';
import { customers } from '@/features/fieldService/sampleData';
import { theme } from '@/theme/theme';

export default function CustomersScreen() {
  return (
    <Screen title="Customers" subtitle="Customer records available in the field." scroll={false}>
      <FlatList
        data={customers}
        keyExtractor={(item) => item.id}
        contentContainerStyle={{ gap: 12 }}
        renderItem={({ item }) => (
          <View style={theme.card}>
            <Text style={theme.cardTitle}>{item.name}</Text>
            <Text style={theme.body}>{item.phone} • {item.email}</Text>
            <Text style={theme.caption}>{item.address}</Text>
          </View>
        )}
      />
    </Screen>
  );
}
`
}

func estimatesScreenTSX() string {
	return `import { useMemo, useState } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';
import { Screen } from '@/components/ui/Screen';
import { queueDraftForSync } from '@/data/syncQueue';
import { theme } from '@/theme/theme';

export default function EstimatesScreen() {
  const [laborHours, setLaborHours] = useState('28');
  const [laborRate, setLaborRate] = useState('85');
  const [materials, setMaterials] = useState('4200');
  const [markup, setMarkup] = useState('32');
  const [saved, setSaved] = useState(false);
  const estimate = useMemo(() => {
    const labor = Number(laborHours || 0) * Number(laborRate || 0);
    const materialCost = Number(materials || 0);
    const subtotal = labor + materialCost;
    const markupAmount = subtotal * (Number(markup || 0) / 100);
    const finalPrice = subtotal + markupAmount;
    const profit = finalPrice - subtotal;
    const margin = finalPrice > 0 ? (profit / finalPrice) * 100 : 0;
    return { labor, materialCost, subtotal, markupAmount, finalPrice, profit, margin };
  }, [laborHours, laborRate, materials, markup]);

  const saveDraft = async () => {
    await queueDraftForSync({ id: 'draft-' + Date.now(), estimate });
    setSaved(true);
  };

  return (
    <Screen title="Estimate Builder" subtitle="Live quote math with offline draft sync.">
      <View style={{ gap: 12 }}>
        {[
          ['Labor hours', laborHours, setLaborHours],
          ['Labor rate', laborRate, setLaborRate],
          ['Materials', materials, setMaterials],
          ['Markup %', markup, setMarkup]
        ].map(([label, value, setValue]) => (
          <TextInput
            key={label as string}
            accessibilityLabel={label as string}
            keyboardType="numeric"
            value={value as string}
            onChangeText={setValue as (value: string) => void}
            style={theme.input}
            placeholder={label as string}
            placeholderTextColor={theme.colors.muted}
          />
        ))}
        <View style={theme.card}>
          <Text style={theme.cardTitle}>Customer price ${Math.round(estimate.finalPrice).toLocaleString()}</Text>
          <Text style={theme.body}>Profit ${Math.round(estimate.profit).toLocaleString()} • Margin {estimate.margin.toFixed(1)}%</Text>
          <Text style={theme.caption}>Subtotal ${Math.round(estimate.subtotal).toLocaleString()} + markup ${Math.round(estimate.markupAmount).toLocaleString()}</Text>
        </View>
        <Pressable accessibilityRole="button" onPress={saveDraft} style={theme.primaryButton}>
          <Text style={theme.primaryButtonText}>{saved ? 'Draft queued for sync' : 'Save offline draft'}</Text>
        </Pressable>
      </View>
    </Screen>
  );
}
`
}

func estimateDetailTSX() string {
	return `import { Text, View } from 'react-native';
import { Screen } from '@/components/ui/Screen';
import { jobs } from '@/features/fieldService/sampleData';
import { theme } from '@/theme/theme';

export default function EstimateDetailModal() {
  const job = jobs[0];
  return (
    <Screen title="Estimate Detail" subtitle="Customer-ready quote preview and crew instructions.">
      <View style={theme.card}>
        <Text style={theme.cardTitle}>{job.title}</Text>
        <Text style={theme.body}>{job.customerName}</Text>
        <Text style={theme.caption}>{job.address}</Text>
      </View>
      <View style={theme.card}>
        <Text style={theme.cardTitle}>Crew instructions</Text>
        <Text style={theme.body}>Verify measurements, capture before photos, and confirm access constraints before material staging.</Text>
      </View>
    </Screen>
  );
}
`
}

func apiClientTS() string {
	return `import Constants from 'expo-constants';
import * as SecureStore from 'expo-secure-store';

type RuntimeConfig = {
  apiBaseUrl?: string;
};

const runtimeConfig = (Constants.expoConfig?.extra ?? {}) as RuntimeConfig;
const API_BASE_URL = runtimeConfig.apiBaseUrl ?? '';

export class ApiError extends Error {
  constructor(message: string, public status: number) {
    super(message);
  }
}

export async function apiRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = await SecureStore.getItemAsync('auth_token');
  const response = await fetch(API_BASE_URL + path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: 'Bearer ' + token } : {}),
      ...(options.headers ?? {})
    }
  });
  if (!response.ok) {
    throw new ApiError('Request failed with status ' + response.status, response.status);
  }
  return response.json() as Promise<T>;
}
`
}

func apiTypesTS() string {
	return `export type AuthSession = {
  token: string;
  user: { id: string; email: string; name: string };
};

export type Job = {
  id: string;
  title: string;
  customerName: string;
  address: string;
  status: string;
  estimatedValue: number;
  syncState: 'synced' | 'pending sync' | 'offline draft';
};

export type EstimateDraft = {
  id: string;
  estimate: Record<string, number>;
};
`
}

func authProviderTSX() string {
	return `import * as SecureStore from 'expo-secure-store';
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

type Session = { email: string; token: string };
type AuthContextValue = {
  session: Session | null;
  bootstrapped: boolean;
  signIn: (email: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [bootstrapped, setBootstrapped] = useState(false);

  useEffect(() => {
    SecureStore.getItemAsync('auth_token').then((token) => {
      if (token) setSession({ email: 'restored@fieldops.test', token });
      setBootstrapped(true);
    });
  }, []);

  const value = useMemo<AuthContextValue>(() => ({
    session,
    bootstrapped,
    signIn: async (email: string) => {
      const token = 'demo-token-' + Date.now();
      await SecureStore.setItemAsync('auth_token', token);
      setSession({ email, token });
    },
    signOut: async () => {
      await SecureStore.deleteItemAsync('auth_token');
      setSession(null);
    }
  }), [bootstrapped, session]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used inside AuthProvider');
  return value;
}
`
}

func emptyStateTSX() string {
	return `import { Text, View } from 'react-native';
import { theme } from '@/theme/theme';

export function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <View style={theme.card}>
      <Text style={theme.cardTitle}>{title}</Text>
      <Text style={theme.body}>{body}</Text>
    </View>
  );
}
`
}

func screenTSX() string {
	return `import { PropsWithChildren } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { theme } from '@/theme/theme';

export function Screen({ title, subtitle, scroll = true, children }: PropsWithChildren<{ title: string; subtitle?: string; scroll?: boolean }>) {
  const insets = useSafeAreaInsets();
  const contentStyle = { paddingTop: insets.top + 24, paddingHorizontal: 18, paddingBottom: insets.bottom + 24 };
  const header = (
    <View style={{ gap: 6, marginBottom: 18 }}>
      <Text style={theme.title}>{title}</Text>
      {subtitle ? <Text style={theme.subtitle}>{subtitle}</Text> : null}
    </View>
  );
  if (!scroll) {
    return (
      <View style={{ flex: 1, backgroundColor: theme.colors.navy, ...contentStyle }}>
        {header}
        {children}
      </View>
    );
  }
  return (
    <ScrollView style={{ flex: 1, backgroundColor: theme.colors.navy }} contentContainerStyle={contentStyle}>
      {header}
      {children}
    </ScrollView>
  );
}
`
}

func localStoreTS() string {
	return `import AsyncStorage from '@react-native-async-storage/async-storage';

export async function saveJSON<T>(key: string, value: T) {
  await AsyncStorage.setItem(key, JSON.stringify(value));
}

export async function loadJSON<T>(key: string, fallback: T): Promise<T> {
  const raw = await AsyncStorage.getItem(key);
  return raw ? JSON.parse(raw) as T : fallback;
}
`
}

func syncQueueTS() string {
	return `import { loadJSON, saveJSON } from './localStore';
import type { EstimateDraft } from '@/api/types';

const QUEUE_KEY = 'pending_estimate_sync';

export async function queueDraftForSync(draft: EstimateDraft) {
  const queue = await loadJSON<EstimateDraft[]>(QUEUE_KEY, []);
  await saveJSON(QUEUE_KEY, [...queue, draft]);
}

export async function listQueuedDrafts() {
  return loadJSON<EstimateDraft[]>(QUEUE_KEY, []);
}
`
}

func sampleDataTS() string {
	return `import type { Job } from '@/api/types';

export const jobs: Job[] = [
  { id: 'job-101', title: 'Roof repair and gutter reset', customerName: 'Mason Ridge HOA', address: '2148 Ridgeview Dr', status: 'Estimate needed', estimatedValue: 18450, syncState: 'pending sync' },
  { id: 'job-102', title: 'Kitchen water damage restoration', customerName: 'Diane Ortega', address: '7712 Summit Ave', status: 'In progress', estimatedValue: 32780, syncState: 'synced' },
  { id: 'job-103', title: 'Exterior paint and trim', customerName: 'Northline Retail', address: '400 Market St', status: 'Proposal sent', estimatedValue: 24600, syncState: 'offline draft' }
];

export const customers = [
  { id: 'cus-1', name: 'Mason Ridge HOA', phone: '(512) 555-0184', email: 'board@masonridge.test', address: '2148 Ridgeview Dr' },
  { id: 'cus-2', name: 'Diane Ortega', phone: '(512) 555-0197', email: 'diane@example.test', address: '7712 Summit Ave' },
  { id: 'cus-3', name: 'Northline Retail', phone: '(512) 555-0162', email: 'ops@northline.test', address: '400 Market St' }
];
`
}

func nativeCapabilitiesTS(spec MobileAppSpec) string {
	capabilities, _ := json.MarshalIndent(spec.Capabilities, "", "  ")
	return fmt.Sprintf(`export const nativeCapabilities = %s as const;

export const nativeCapabilityNotes = {
  preview: 'Expo Web preview is browser-rendered and does not prove native permission behavior.',
  build: 'Camera, notifications, and native storage must be validated in a development or EAS build.'
};
`, string(capabilities))
}

func themeTS() string {
	return `import { TextStyle, ViewStyle } from 'react-native';

const colors = {
  navy: '#0F172A',
  panel: '#111C33',
  panelSoft: '#17223A',
  cyan: '#22D3EE',
  text: '#E5F3FF',
  muted: '#8EA3B8',
  border: 'rgba(34, 211, 238, 0.22)',
  danger: '#F87171'
};

export const theme = {
  colors,
  title: { color: colors.text, fontSize: 32, fontWeight: '800', letterSpacing: -0.8 } as TextStyle,
  subtitle: { color: colors.muted, fontSize: 15, lineHeight: 22 } as TextStyle,
  body: { color: colors.text, fontSize: 15, lineHeight: 22 } as TextStyle,
  caption: { color: colors.muted, fontSize: 13, marginTop: 8 } as TextStyle,
  cardTitle: { color: colors.text, fontSize: 17, fontWeight: '700' } as TextStyle,
  badge: { color: colors.cyan, fontSize: 12, fontWeight: '700', textTransform: 'uppercase' } as TextStyle,
  card: { backgroundColor: colors.panel, borderColor: colors.border, borderWidth: 1, borderRadius: 22, padding: 16 } as ViewStyle,
  input: { backgroundColor: colors.panelSoft, borderColor: colors.border, borderWidth: 1, borderRadius: 16, color: colors.text, paddingHorizontal: 14, paddingVertical: 13, fontSize: 16 } as TextStyle,
  primaryButton: { minHeight: 52, alignItems: 'center', justifyContent: 'center', borderRadius: 18, backgroundColor: colors.cyan } as ViewStyle,
  primaryButtonText: { color: colors.navy, fontWeight: '800', fontSize: 16 } as TextStyle
};
`
}
