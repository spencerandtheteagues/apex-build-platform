import React from 'react'
import {
  AlertTriangle,
  BadgeDollarSign,
  FileCheck2,
  Lock,
  Scale,
} from 'lucide-react'

export const LEGAL_POLICY_VERSION = '2026-03-21'

export type LegalDocumentId =
  | 'terms'
  | 'privacy'
  | 'acceptable-use'
  | 'billing'
  | 'ai-policy'

export type LegalDocument = {
  id: LegalDocumentId
  title: string
  summary: string
  icon: React.ComponentType<{ className?: string }>
  sections: Array<{
    heading: string
    paragraphs: string[]
  }>
}

export const LEGAL_DOCUMENTS: LegalDocument[] = [
  {
    id: 'terms',
    title: 'Terms of Service',
    summary: 'Platform access, arbitration, warranty disclaimers, and liability limits.',
    icon: Scale,
    sections: [
      {
        heading: 'Platform Contract',
        paragraphs: [
          'APEX.BUILD is a hosted software platform for code generation, collaboration, deployment tooling, and related AI-assisted workflows. By creating an account, signing in, or using the service, you agree to these terms and all policies referenced alongside them.',
          'You must be legally able to enter a binding contract, use the service only for lawful business or personal purposes, and keep your account credentials secure. You are responsible for all activity that occurs under your account.',
        ],
      },
      {
        heading: 'Ownership and Responsibility',
        paragraphs: [
          'You retain ownership of content, prompts, code, files, and project materials you submit, subject to the rights needed for APEX.BUILD to host, process, secure, analyze, and transmit that material while operating the service.',
          'You are responsible for reviewing all generated output before use in production. APEX.BUILD does not guarantee that generated code, deployments, suggestions, or automations are accurate, secure, lawful, merchantable, or fit for any particular purpose.',
        ],
      },
      {
        heading: 'Risk Allocation',
        paragraphs: [
          'The service is provided on an as-is and as-available basis. To the maximum extent permitted by law, APEX.BUILD disclaims warranties of any kind, whether express, implied, statutory, or otherwise.',
          'To the maximum extent permitted by law, APEX.BUILD, its operators, contractors, and affiliates will not be liable for indirect, incidental, special, consequential, exemplary, or punitive damages, or for lost profits, revenue, goodwill, data, business interruption, security incidents caused by user actions, or downstream third-party claims arising from user content or generated output.',
          'Any claim relating to the service must be brought on an individual basis, not as a class, consolidated, or representative action, and unresolved disputes should be submitted to binding arbitration unless non-waivable law requires otherwise.',
        ],
      },
    ],
  },
  {
    id: 'privacy',
    title: 'Privacy Policy',
    summary: 'What data the app handles, why it is used, and how users can request changes.',
    icon: Lock,
    sections: [
      {
        heading: 'Information We Process',
        paragraphs: [
          'APEX.BUILD processes account details, authentication data, billing records, project files, prompts, generated output, logs, usage metadata, device/browser information, and support communications as needed to deliver the service.',
          'Sensitive integrations such as API keys, environment variables, and deployment credentials may be encrypted or masked where supported, but you should only provide credentials you are authorized to use.',
        ],
      },
      {
        heading: 'Why We Use Data',
        paragraphs: [
          'We use data to authenticate users, operate the product, prevent abuse, process billing, debug incidents, enforce platform policies, improve reliability, and comply with legal obligations.',
          'We may use service metadata and operational telemetry to detect fraud, investigate misuse, defend legal claims, and maintain security. We do not promise that deleted content can always be recovered after removal requests or retention expiry.',
        ],
      },
      {
        heading: 'Retention, Sharing, and Requests',
        paragraphs: [
          'Data may be shared with processors and infrastructure providers that support hosting, payments, logging, analytics, AI inference, customer support, and security monitoring, but only to the extent needed to operate the service.',
          'Users can contact support@apex.build for privacy requests, subject to identity verification, security requirements, and lawful retention duties.',
        ],
      },
    ],
  },
  {
    id: 'acceptable-use',
    title: 'Acceptable Use Policy',
    summary: 'Prohibited activities, abuse handling, and account enforcement rules.',
    icon: AlertTriangle,
    sections: [
      {
        heading: 'Prohibited Conduct',
        paragraphs: [
          'You may not use APEX.BUILD to violate laws, infringe intellectual property, deploy malware, phish credentials, exfiltrate data, evade security controls, abuse third-party services, conduct unauthorized penetration testing, generate exploit payloads for unlawful use, or store or process unlawful content.',
          'You may not interfere with service availability, bypass quotas, scrape the platform without permission, resell access without authorization, or use the product in a way that creates disproportionate legal, regulatory, or infrastructure risk.',
        ],
      },
      {
        heading: 'Enforcement',
        paragraphs: [
          'APEX.BUILD may monitor for abuse indicators, rate-limit or suspend accounts, restrict features, remove content, preserve evidence, and cooperate with payment processors, hosting providers, or law enforcement where required.',
          'Violations may result in immediate suspension or termination without notice, credit forfeiture where permitted, and referral of claims or losses back to the responsible user.',
        ],
      },
    ],
  },
  {
    id: 'billing',
    title: 'Billing, Credits, and Refunds',
    summary: 'Subscription terms, prepaid credits, renewal behavior, and refund boundaries.',
    icon: BadgeDollarSign,
    sections: [
      {
        heading: 'Charges and Renewals',
        paragraphs: [
          'Paid plans, usage-based charges, prepaid credits, and add-on services may renew automatically until canceled. You authorize APEX.BUILD and its payment providers to charge the payment method on file for subscriptions, metered usage, taxes, and overdue balances.',
          'Credits, promotional balances, and included usage allotments have no cash value unless non-waivable law requires otherwise and may expire or reset under the applicable plan rules.',
        ],
      },
      {
        heading: 'Refund Policy',
        paragraphs: [
          'Except where required by law or expressly promised in writing, subscription fees, metered usage, consumed credits, processor fees, and completed AI or deployment work are non-refundable.',
          'Chargebacks, payment disputes, or repeated failed payments may trigger account suspension, collections activity, or loss of access until the issue is resolved.',
        ],
      },
    ],
  },
  {
    id: 'ai-policy',
    title: 'AI and Content Policy',
    summary: 'Generated-output disclaimers, user review duties, and moderation expectations.',
    icon: FileCheck2,
    sections: [
      {
        heading: 'Generated Output',
        paragraphs: [
          'APEX.BUILD may produce code, architecture suggestions, deployment plans, copy, and other machine-generated output. Generated output can be incomplete, insecure, biased, inaccurate, or legally problematic, and must be reviewed by a qualified human before use.',
          'You are solely responsible for testing, licensing review, dependency review, security review, privacy review, export-control review, and production approvals related to generated output or uploaded materials.',
        ],
      },
      {
        heading: 'Content Review and Takedowns',
        paragraphs: [
          'APEX.BUILD may restrict content, prompts, or generated outputs that create legal, safety, abuse, or platform-integrity risk. We may remove or disable access to content in response to rights-holder complaints, safety concerns, or processor and provider requirements.',
          'Questions, notices, and legal complaints should be sent to support@apex.build with enough detail to identify the account, project, affected material, and requested action.',
        ],
      },
    ],
  },
]
