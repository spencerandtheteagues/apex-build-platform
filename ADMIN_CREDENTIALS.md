# APEX.BUILD Admin Credentials

## Primary Admin Account
- **Email:** spencerandtheteagues@gmail.com
- **Password:** THE$T@R$H1PKEY!
- **Role:** Super Admin

## Backup Admin Account (To Create)
- **Username:** admin
- **Email:** admin@apex.build
- **Password:** ApexAdmin2026!
- **Role:** Admin

## API Endpoints
- **Backend:** https://apex-backend-y42k.onrender.com
- **Frontend:** https://apex-frontend-gigq.onrender.com

## Current Status
**BACKEND IS DOWN** - All API routes returning 404 "Not Found"

This needs to be fixed before any authentication will work.

## To Fix Backend:
1. Check Render dashboard for deployment errors
2. Verify the backend service is running
3. Check logs for startup errors
4. Ensure DATABASE_URL is properly set
5. Ensure all environment variables are configured

## Environment Variables Required:
- DATABASE_URL
- JWT_SECRET
- ANTHROPIC_API_KEY
- OPENAI_API_KEY
- GEMINI_API_KEY
- SECRETS_MASTER_KEY
- STRIPE_SECRET_KEY (optional)
