import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../auth_state.dart';
import '../theme.dart';
import '../widgets/auth_widgets.dart';

class RegisterScreen extends StatefulWidget {
  final VoidCallback onGoLogin;
  const RegisterScreen({super.key, required this.onGoLogin});

  @override
  State<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends State<RegisterScreen> {
  final _formKey = GlobalKey<FormState>();
  final _userCtrl = TextEditingController();
  final _emailCtrl = TextEditingController();
  final _passCtrl = TextEditingController();
  final _confirmCtrl = TextEditingController();
  bool _obscure = true;

  @override
  void dispose() {
    _userCtrl.dispose();
    _emailCtrl.dispose();
    _passCtrl.dispose();
    _confirmCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    await context.read<AuthState>()
        .register(_userCtrl.text.trim(), _emailCtrl.text.trim(), _passCtrl.text);
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthState>();
    return Scaffold(
      backgroundColor: kBg,
      body: Stack(children: [
        Positioned.fill(child: CustomPaint(painter: const AuthBgPainter())),
        Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Column(mainAxisSize: MainAxisSize.min, children: [
                const VeltraLogo(),
                const SizedBox(height: 36),
                Container(
                  padding: const EdgeInsets.all(28),
                  decoration: BoxDecoration(
                    color: kSurface.withOpacity(0.95),
                    borderRadius: BorderRadius.circular(18),
                    border: Border.all(color: kBrand.withOpacity(0.3)),
                    boxShadow: [
                      BoxShadow(color: kBrand.withOpacity(0.08), blurRadius: 40),
                    ],
                  ),
                  child: Form(
                    key: _formKey,
                    child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                      Text('Criar conta',
                          style: Theme.of(context).textTheme.titleLarge?.copyWith(fontSize: 20)),
                      const SizedBox(height: 4),
                      const Text('Comece a negociar agora',
                          style: TextStyle(color: kTxtSub, fontSize: 13)),
                      const SizedBox(height: 24),
                      TextFormField(
                        controller: _userCtrl,
                        style: const TextStyle(color: kTxt),
                        decoration: const InputDecoration(
                          labelText: 'Usuário',
                          hintText: 'mín. 3 chars',
                          prefixIcon: Icon(Icons.person_outline, size: 18, color: kTxtSub),
                        ),
                        textInputAction: TextInputAction.next,
                        validator: (v) {
                          if (v == null || v.trim().length < 3) return 'Mínimo 3 caracteres';
                          if (v.contains(' ')) return 'Sem espaços';
                          return null;
                        },
                      ),
                      const SizedBox(height: 12),
                      TextFormField(
                        controller: _emailCtrl,
                        style: const TextStyle(color: kTxt),
                        decoration: const InputDecoration(
                          labelText: 'E-mail',
                          prefixIcon: Icon(Icons.email_outlined, size: 18, color: kTxtSub),
                        ),
                        keyboardType: TextInputType.emailAddress,
                        textInputAction: TextInputAction.next,
                        validator: (v) {
                          if (v == null || v.trim().isEmpty) return 'Obrigatório';
                          if (!v.contains('@')) return 'E-mail inválido';
                          return null;
                        },
                      ),
                      const SizedBox(height: 12),
                      TextFormField(
                        controller: _passCtrl,
                        obscureText: _obscure,
                        style: const TextStyle(color: kTxt),
                        decoration: InputDecoration(
                          labelText: 'Senha',
                          hintText: 'mín. 8 caracteres',
                          prefixIcon: const Icon(Icons.lock_outline, size: 18, color: kTxtSub),
                          suffixIcon: IconButton(
                            icon: Icon(
                                _obscure ? Icons.visibility_off_outlined : Icons.visibility_outlined,
                                size: 18, color: kTxtSub),
                            onPressed: () => setState(() => _obscure = !_obscure),
                          ),
                        ),
                        textInputAction: TextInputAction.next,
                        validator: (v) {
                          if (v == null || v.length < 8) return 'Mínimo 8 caracteres';
                          return null;
                        },
                      ),
                      const SizedBox(height: 12),
                      TextFormField(
                        controller: _confirmCtrl,
                        obscureText: _obscure,
                        style: const TextStyle(color: kTxt),
                        decoration: const InputDecoration(
                          labelText: 'Confirmar senha',
                          prefixIcon: Icon(Icons.lock_outline, size: 18, color: kTxtSub),
                        ),
                        textInputAction: TextInputAction.done,
                        onFieldSubmitted: (_) => _submit(),
                        validator: (v) {
                          if (v != _passCtrl.text) return 'Senhas não coincidem';
                          return null;
                        },
                      ),
                      if (auth.error != null) ...[
                        const SizedBox(height: 12),
                        AuthErrorBanner(auth.error!),
                      ],
                      const SizedBox(height: 20),
                      GlowButton(
                          onPressed: auth.loading ? null : _submit,
                          loading: auth.loading,
                          label: 'Criar conta'),
                    ]),
                  ),
                ),
                const SizedBox(height: 20),
                Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                  const Text('Já tem conta?',
                      style: TextStyle(color: kTxtSub, fontSize: 13)),
                  const SizedBox(width: 6),
                  GestureDetector(
                    onTap: widget.onGoLogin,
                    child: const Text('Entrar',
                        style: TextStyle(color: kBrand, fontSize: 13, fontWeight: FontWeight.w600)),
                  ),
                ]),
              ]),
            ),
          ),
        ),
      ]),
    );
  }
}
